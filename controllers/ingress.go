package controllers

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileIngress configures external routing and ingress endpoints
func (r *PreviewEnvironmentReconciler) reconcileIngress(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) ([]idpv1alpha1.EndpointStatus, error) {
	if !previewEnv.Spec.Ingress.Enabled || len(previewEnv.Spec.Workloads) == 0 {
		return nil, nil
	}

	ingressName := fmt.Sprintf("%s-ingress", previewEnv.Name)
	host, err := r.renderHost(previewEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to render ingress host: %w", err)
	}

	pathType := networkingv1.PathTypePrefix
	primaryWorkload := previewEnv.Spec.Workloads[0]

	ingress := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: ingressName}, ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			path := "/"
			if previewEnv.Spec.Ingress.Path != "" {
				path = previewEnv.Spec.Ingress.Path
			}

			ingressClassName := previewEnv.Spec.Ingress.IngressClassName
			if ingressClassName == "" {
				ingressClassName = "nginx"
			}

			newIngress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ingressName,
					Namespace: targetNamespace,
					Labels: map[string]string{
						ManagedByLabelKey:   ManagedByLabelVal,
						EnvironmentLabelKey: previewEnv.Name,
					},
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": ingressClassName,
						"cert-manager.io/cluster-issuer": "letsencrypt-prod",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &ingressClassName,
					Rules: []networkingv1.IngressRule{
						{
							Host: host,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     path,
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: primaryWorkload.Name,
													Port: networkingv1.ServiceBackendPort{
														Number: primaryWorkload.Port,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			if previewEnv.Spec.Ingress.TLS {
				newIngress.Spec.TLS = []networkingv1.IngressTLS{
					{
						Hosts:      []string{host},
						SecretName: fmt.Sprintf("%s-tls", ingressName),
					},
				}
			}

			if err := r.Create(ctx, newIngress); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	protocol := "http"
	if previewEnv.Spec.Ingress.TLS {
		protocol = "https"
	}

	endpoints := []idpv1alpha1.EndpointStatus{
		{
			Name: primaryWorkload.Name,
			URL:  fmt.Sprintf("%s://%s", protocol, host),
			Port: primaryWorkload.Port,
		},
	}

	return endpoints, nil
}

type hostContext struct {
	PRNumber  int32
	Branch    string
	RawBranch string
	Repo      string
}

func sanitizeDNSLabel(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = s[:63]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		return "env"
	}
	return s
}

func (r *PreviewEnvironmentReconciler) renderHost(previewEnv *idpv1alpha1.PreviewEnvironment) (string, error) {
	tmplStr := previewEnv.Spec.Ingress.HostTemplate
	if tmplStr == "" {
		tmplStr = "pr-{{.PRNumber}}.preview.cloudnative.io"
	}

	tmpl, err := template.New("host").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	ctx := hostContext{
		PRNumber:  previewEnv.Spec.PRNumber,
		Branch:    sanitizeDNSLabel(previewEnv.Spec.Branch),
		RawBranch: previewEnv.Spec.Branch,
		Repo:      previewEnv.Spec.Repository,
	}

	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}
