# tfclean

## Install

```bash
go install github.com/takaishi/tfclean/cmd/tfclean
```

## Usage

```bash
% AWS_PROFILE=xxxxxxx tfclean --tfstate s3://path/to/tfstate /path/to/tffiles
```

## Features

- Blocks
  - [x] Remove moved blocks that is applied.
  - [x] Remove import blocks that is applied.
  - [ ] Remove removed blocks that is applied.
  - [ ] Forcefully remove all moved/import/removed blocks.
- Location of tfstate
  - [x] S3
  - [ ] Local

## GitHub Actions

This is example of GitHub Actions for creating automatically pull request with tfclean. I recommend to use GitHub App to generate token.

```yaml
name: tfclean

on:
  push:
    branches:
      - main

permissions:
  pull-requests: write # This is required for creating pull request for auto-remove blocks by tfclean

jobs:
  tfclean:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/create-github-app-token@v1
        id: app-token
        with:
          app-id: ${{ secrets.GITHUB_APP_ID }}
          private-key: ${{ secrets.GITHUB_APP_PRIVATE_KEY }}
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: "aws_role_arn_for_oidc"
          aws-region: "ap-northeast-1"
      - name: install tfclean
        run: |
          cd /tmp/
          curl -sL https://github.com/takaishi/tfclean/releases/download/v0.0.3/tfclean_Linux_x86_64.tar.gz --output tfclean_Linux_x86_64.tar.gz
          tar xvzf ./tfclean_Linux_x86_64.tar.gz
          sudo mv tfclean /usr/local/bin/
      - name: run tfclean
        run: /usr/local/bin/tfclean --tfstate s3://path/to/tfstate .
      - name: Check changes
        id: diff-check
        run: git diff --exit-code || echo "changes_detected=true" >> $GITHUB_OUTPUT
      - name: Commit changes
        if: steps.diff-check.outputs.changes_detected == 'true'
        run: |
          echo steps.diff-check.outputs.changes_detected: ${{ steps.diff-check.outputs.changes_detected }}
          branch_name=tfclean_$(date +"%Y%m%d%H%M")
          git switch -c ${branch_name}
          git config --global user.email "EMAIL"
          git config --global user.name "NAME"
          git add .
          git diff --cached --exit-code || (git commit -m "chore: auto-remove blocks by tfclean" && git push origin ${branch_name})
          gh pr create --base staging --head ${branch_name} --title "auto-remove blocks by tfclean" --body ""
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
```
