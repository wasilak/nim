# Command Reference

## Setup

```bash
nim init              # Initialize configuration directory
nim doctor            # Check system prerequisites
```

## Core workflow

```bash
nim plan                    # Show what would change (dry-run)
nim plan --diff             # Show inline file diffs
nim plan --target KIND/name # Limit to a specific resource
nim plan --target /pattern/ # Target with regex (e.g., /Brew.*/)
nim apply                   # Dry-run apply (default)
nim apply --confirm         # Actually apply changes
nim apply --diff            # Show diffs during apply
```

## State management

```bash
nim state list                                         # Show all managed resources
nim state list --output json                           # JSON output
nim state import HomeBrewPackages/core-tools[ripgrep]  # Import existing into state
nim state move HomeBrewPackages/old[pkg] HomeBrewPackages/new[pkg]  # Move item between groups
nim state remove HomeBrewPackages/core-tools[ripgrep]  # Remove from state
nim state pull                                          # Download state from S3 backend
nim state push                                          # Upload state to S3 backend
```

## Other

```bash
nim version     # Print version
nim stats        # Show resource statistics
nim stats --all  # Include coverage (installed vs tracked)
```