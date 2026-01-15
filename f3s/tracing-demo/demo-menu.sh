#!/bin/bash
# Argo Rollouts Demo Menu - Choose demo scenario

BOLD='\033[1m'
BLUE='\033[0;34m'
GREEN='\033[0;32m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_menu() {
    clear
    echo -e "${BOLD}Argo Rollouts Demo Menu${NC}"
    echo ""
    echo "Choose a demo:"
    echo ""
    echo "  1) Run full canary rollout demo"
    echo "     • Triggers rollout"
    echo "     • Monitors 0-15s (canary), 15-60s (observe), 60-90s (promote)"
    echo "     • Shows final state"
    echo ""
    echo "  2) Abort rollout demo"
    echo "     • Triggers rollout"
    echo "     • Waits 20s for canary to be ready"
    echo "     • Aborts mid-canary"
    echo "     • Shows rollback behavior"
    echo ""
    echo "  3) Reset rollout"
    echo "     • Aborts any in-progress rollout"
    echo "     • Removes demo env vars"
    echo "     • Returns to clean state"
    echo ""
    echo "  4) Check rollout status"
    echo "     • Shows current state"
    echo "     • Shows pod replicas"
    echo "     • Shows recent history"
    echo ""
    echo "  5) Watch rollout (real-time)"
    echo "     • Opens live rollout viewer"
    echo ""
    echo "  0) Exit"
    echo ""
}

check_status() {
    echo ""
    echo -e "${BOLD}Rollout Status:${NC}"
    kubectl argo rollouts status tracing-demo-frontend -n services
    echo ""
    echo -e "${BOLD}Pod Replicas:${NC}"
    kubectl get pods -n services -l app=tracing-demo-frontend -o wide --no-headers | awk '{print $1, $3, $4}'
    echo ""
    echo -e "${BOLD}Recent Revisions:${NC}"
    kubectl argo rollouts history tracing-demo-frontend -n services | head -5
    echo ""
    read -p "Press enter to continue..."
}

watch_live() {
    echo ""
    echo -e "${BOLD}Live Rollout Viewer (Ctrl+C to exit)${NC}"
    echo ""
    kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
}

while true; do
    show_menu
    read -p "Select option: " choice
    
    case $choice in
        1)
            bash "$SCRIPT_DIR/demo-canary-rollout.sh"
            ;;
        2)
            bash "$SCRIPT_DIR/demo-abort-rollout.sh"
            ;;
        3)
            bash "$SCRIPT_DIR/demo-reset.sh"
            ;;
        4)
            check_status
            ;;
        5)
            watch_live
            ;;
        0)
            echo "Goodbye!"
            exit 0
            ;;
        *)
            echo "Invalid option"
            sleep 1
            ;;
    esac
done
