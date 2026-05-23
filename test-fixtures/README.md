# Test Fixtures

This directory contains sample terragrunt configurations for e2e testing.

## Structure

```
test-fixtures/
├── README.md                  # This file
└── terragrunt/
    ├── prod/
    │   └── terragrunt.hcl     # Production workload config
    └── modules/
        └── example/
            └── main.tf        # Sample Terraform module
```

## Purpose

These fixtures are used for end-to-end testing of the flux-terragrunt-controller.
The controller monitors a GitRepository for changes to paths matching `prod/` filter,
and when changes are detected, it spawns a runner job to execute `terragrunt run --filter prod --all plan`.

## Usage in E2E Tests

1. Push this repository to a Git hosting service (GitHub, GitLab, etc.)
2. Configure the e2e test to reference your repository URL
3. The controller will detect changes and spawn terragrunt runner jobs

## Environment Variables for Testing

The following environment variables control the test repository:

- `TEST_GIT_REPO_URL` - Git repository URL containing terragrunt code
- `TEST_GIT_BRANCH` - Branch to monitor (default: main)
- `TEST_GIT_SECRET_REF` - Optional secret name for private repositories
