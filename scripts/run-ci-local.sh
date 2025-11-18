#!/usr/bin/env bash

# Script to run GitHub Actions workflows locally using act
# This automates the local CI pipeline testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
JOB=""
WORKFLOW=".github/workflows/tests.yml"
VERBOSE=false
LIST_JOBS=false
DRY_RUN=false
EVENT="push"
PLATFORM="ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest"

# Help message
show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Run GitHub Actions workflows locally using act.

OPTIONS:
    -j, --job JOB           Run specific job (default: all jobs)
    -w, --workflow FILE     Workflow file to run (default: tests.yml)
    -v, --verbose           Enable verbose output
    -l, --list              List available jobs and exit
    -n, --dry-run          Show what would be run without executing
    -e, --event EVENT       Event type: push, pull_request (default: push)
    -p, --platform PLATFORM Custom platform mapping
    -h, --help             Show this help message

EXAMPLES:
    # Run all jobs in the default workflow
    $(basename "$0")

    # Run specific job
    $(basename "$0") -j test

    # List available jobs
    $(basename "$0") -l

    # Run with verbose output
    $(basename "$0") -v

    # Simulate pull request event
    $(basename "$0") -e pull_request

    # Dry run to see what would be executed
    $(basename "$0") -n

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -j|--job)
                JOB="$2"
                shift 2
                ;;
            -w|--workflow)
                WORKFLOW="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -l|--list)
                LIST_JOBS=true
                shift
                ;;
            -n|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -e|--event)
                EVENT="$2"
                shift 2
                ;;
            -p|--platform)
                PLATFORM="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                echo -e "${RED}Error: Unknown option $1${NC}"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check if act is installed
check_act_installed() {
    if ! command -v act &> /dev/null; then
        echo -e "${RED}Error: 'act' is not installed${NC}"
        echo -e "${YELLOW}Install it with: mise install act${NC}"
        exit 1
    fi
}

# List available jobs
list_jobs() {
    echo -e "${BLUE}Listing jobs in workflow: ${WORKFLOW}${NC}"
    act -l -W "${WORKFLOW}"
}

# Build act command
build_act_command() {
    local cmd="act"

    # Add event type
    cmd="${cmd} ${EVENT}"

    # Add workflow file
    cmd="${cmd} -W ${WORKFLOW}"

    # Add platform
    cmd="${cmd} -P ${PLATFORM}"

    # Add job if specified
    if [[ -n "${JOB}" ]]; then
        cmd="${cmd} -j ${JOB}"
    fi

    # Add verbose flag
    if [[ "${VERBOSE}" == true ]]; then
        cmd="${cmd} -v"
    fi

    echo "${cmd}"
}

# Run act
run_act() {
    local cmd
    cmd=$(build_act_command)

    echo -e "${BLUE}Running local CI pipeline...${NC}"
    echo -e "${YELLOW}Command: ${cmd}${NC}"
    echo ""

    if [[ "${DRY_RUN}" == true ]]; then
        echo -e "${GREEN}Dry run mode - command would be executed but not running${NC}"
        exit 0
    fi

    # Execute the command
    eval "${cmd}"

    local exit_code=$?

    if [[ ${exit_code} -eq 0 ]]; then
        echo ""
        echo -e "${GREEN}✓ CI pipeline completed successfully${NC}"
    else
        echo ""
        echo -e "${RED}✗ CI pipeline failed with exit code ${exit_code}${NC}"
        exit ${exit_code}
    fi
}

# Main execution
main() {
    parse_args "$@"
    check_act_installed

    if [[ "${LIST_JOBS}" == true ]]; then
        list_jobs
        exit 0
    fi

    run_act
}

main "$@"
