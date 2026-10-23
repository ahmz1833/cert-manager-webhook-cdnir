package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		GroupName = "acme.cdn.ir"
	}

	cmd.RunWebhookServer(GroupName,
		&cdnirDNSProviderSolver{},
	)
}

type cdnirDNSProviderSolver struct {
	client kubernetes.Interface
}

type cdnirDNSProviderConfig struct {
	APIEndpoint       string                   `json:"apiEndpoint,omitempty"`
	APITokenSecretRef corev1.SecretKeySelector `json:"apiTokenSecretRef,omitempty"`
}

func (c *cdnirDNSProviderSolver) Name() string {
	return "cdnir-solver"
}

func (c *cdnirDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load webhook config: %w", err)
	}

	apiToken, err := c.resolveToken(ch.ResourceNamespace, cfg)
	if err != nil {
		return fmt.Errorf("failed to resolve CDN.ir API token: %w", err)
	}

	client := NewCDNirClient(cfg.APIEndpoint, apiToken)

	zone := ch.ResolvedZone
	recordName := extractRecordName(ch.ResolvedFQDN, zone)

	klog.Infof("Presenting ACME challenge: zone=%s, recordName=%s, fqdn=%s", zone, recordName, ch.ResolvedFQDN)

	err = client.AddRecord(zone, recordName, "TXT", ch.Key, 120)
	if err != nil {
		return fmt.Errorf("failed to add TXT record to CDN.ir: %w", err)
	}

	klog.Infof("Successfully presented ACME challenge for %s", ch.ResolvedFQDN)
	return nil
}

func (c *cdnirDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to load webhook config: %w", err)
	}

	apiToken, err := c.resolveToken(ch.ResourceNamespace, cfg)
	if err != nil {
		return fmt.Errorf("failed to resolve CDN.ir API token: %w", err)
	}

	client := NewCDNirClient(cfg.APIEndpoint, apiToken)

	zone := ch.ResolvedZone
	targetFQDN := strings.TrimRight(ch.ResolvedFQDN, ".")

	klog.Infof("Cleaning up ACME challenge: zone=%s, fqdn=%s", zone, targetFQDN)

	records, err := client.ListRecords(zone, "TXT")
	if err != nil {
		return fmt.Errorf("failed to list TXT records for zone %s: %w", zone, err)
	}

	for _, rec := range records {
		recFQDN := strings.TrimRight(rec.Name, ".")
		cleanContent := strings.Trim(rec.Content, "\"")
		cleanKey := strings.Trim(ch.Key, "\"")

		if strings.EqualFold(recFQDN, targetFQDN) && cleanContent == cleanKey {
			klog.Infof("Deleting TXT record id=%d for %s", rec.ID, rec.Name)
			if err := client.DeleteRecord(zone, rec.ID); err != nil {
				return fmt.Errorf("failed to delete TXT record ID %d: %w", rec.ID, err)
			}
		}
	}

	return nil
}

func (c *cdnirDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	c.client = cl
	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (cdnirDNSProviderConfig, error) {
	cfg := cdnirDNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %w", err)
	}
	return cfg, nil
}

func (c *cdnirDNSProviderSolver) resolveToken(namespace string, cfg cdnirDNSProviderConfig) (string, error) {
	// 1. Try to read from Kubernetes Secret if configured and client is available
	if cfg.APITokenSecretRef.Name != "" && c.client != nil {
		ns := namespace
		if ns == "" {
			ns = "cert-manager"
		}
		secret, err := c.client.CoreV1().Secrets(ns).Get(context.Background(), cfg.APITokenSecretRef.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to fetch secret '%s/%s': %w", ns, cfg.APITokenSecretRef.Name, err)
		}

		key := cfg.APITokenSecretRef.Key
		if key == "" {
			key = "token"
		}

		tokenBytes, exists := secret.Data[key]
		if !exists {
			return "", fmt.Errorf("key '%s' not found in secret '%s/%s'", key, ns, cfg.APITokenSecretRef.Name)
		}
		return strings.TrimSpace(string(tokenBytes)), nil
	}

	// 2. Fallback to pod environment variables
	if envToken := os.Getenv("CDNIR_API_TOKEN"); envToken != "" {
		return envToken, nil
	}
	if envToken := os.Getenv("ASIATECH_API_TOKEN"); envToken != "" {
		return envToken, nil
	}
	if envToken := os.Getenv("TOKEN"); envToken != "" {
		return envToken, nil
	}

	return "", fmt.Errorf("no API token found in secret or environment variables")
}

func extractRecordName(fqdn, zone string) string {
	f := strings.TrimRight(strings.TrimSpace(fqdn), ".")
	z := strings.TrimRight(strings.TrimSpace(zone), ".")

	if f == "" || f == z {
		return "@"
	}

	suffix := "." + z
	if strings.HasSuffix(f, suffix) {
		rel := strings.TrimSuffix(f, suffix)
		if rel != "" {
			return rel
		}
		return "@"
	}

	return f
}
