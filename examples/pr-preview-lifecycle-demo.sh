#!/usr/bin/env bash
# ==============================================================================
# Ephemeral Environment Lifecycle Demo Script
# Demonstrates PR creation -> Preview Environment Provisioning -> Verification -> Teardown
# ==============================================================================

set -euo pipefail

COLOR_RESET="\033[0m"
COLOR_GREEN="\033[32m"
COLOR_CYAN="\033[36m"
COLOR_YELLOW="\033[33m"

echo -e "${COLOR_CYAN}================================================================${COLOR_RESET}"
echo -e "${COLOR_CYAN}  Ephemeral Environment Operator — Automated IDP Preview Demo   ${COLOR_RESET}"
echo -e "${COLOR_CYAN}================================================================${COLOR_RESET}"

PR_NUM=${1:-104}
CRD_NAME="preview-pr-${PR_NUM}"
NAMESPACE="preview-pr-${PR_NUM}"

echo -e "\n${COLOR_YELLOW}1. Simulating GitHub Pull Request #${PR_NUM} Opened...${COLOR_RESET}"
echo "Applying PreviewEnvironment CRD definition..."
kubectl apply -f config/samples/full-preview-env.yaml

echo -e "\n${COLOR_YELLOW}2. Waiting for Operator to reconcile and allocate Namespace: ${NAMESPACE}...${COLOR_RESET}"
kubectl wait --for=condition=NamespaceReady "previewenvironments.idp.cloudnative.io/${CRD_NAME}" --timeout=30s || true

echo -e "\n${COLOR_GREEN}✓ Isolated Namespace provisioned:${COLOR_RESET}"
kubectl get namespace "${NAMESPACE}" --show-labels

echo -e "\n${COLOR_YELLOW}3. Inspecting Zero-Trust NetworkPolicy and ResourceQuotas...${COLOR_RESET}"
kubectl get networkpolicy -n "${NAMESPACE}"
kubectl get resourcequota -n "${NAMESPACE}"

echo -e "\n${COLOR_YELLOW}4. Inspecting Workload & Ephemeral Mock Database status...${COLOR_RESET}"
kubectl get deployments,services -n "${NAMESPACE}"

echo -e "\n${COLOR_YELLOW}5. Checking Ingress Routing Endpoints...${COLOR_RESET}"
kubectl get ingress -n "${NAMESPACE}"

echo -e "\n${COLOR_GREEN}✓ PreviewEnvironment Status Summary:${COLOR_RESET}"
kubectl get previewenvironments.idp.cloudnative.io "${CRD_NAME}" -o wide

echo -e "\n${COLOR_CYAN}Demo completed successfully!${COLOR_RESET}"
