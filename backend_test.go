package tfclean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestApp_detectBackendFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(string) error
		want      string
		wantErr   bool
	}{
		{
			name: "S3 backend detected",
			setupFunc: func(dir string) error {
				content := `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "terraform.tfstate"
    region = "us-east-1"
  }
}
`
				return os.WriteFile(filepath.Join(dir, "backend.tf"), []byte(content), 0644)
			},
			want:    "s3://my-bucket/terraform.tfstate",
			wantErr: false,
		},
		{
			name: "S3 backend with path in key",
			setupFunc: func(dir string) error {
				content := `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "path/to/terraform.tfstate"
  }
}
`
				return os.WriteFile(filepath.Join(dir, "backend.tf"), []byte(content), 0644)
			},
			want:    "s3://my-bucket/path/to/terraform.tfstate",
			wantErr: false,
		},
		{
			name: "No backend block",
			setupFunc: func(dir string) error {
				content := `
terraform {
  required_version = ">= 1.0"
}
`
				return os.WriteFile(filepath.Join(dir, "terraform.tf"), []byte(content), 0644)
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "No terraform block",
			setupFunc: func(dir string) error {
				content := `
resource "null_resource" "test" {}
`
				return os.WriteFile(filepath.Join(dir, "main.tf"), []byte(content), 0644)
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Multiple terraform blocks - use first backend",
			setupFunc: func(dir string) error {
				content1 := `
terraform {
  backend "s3" {
    bucket = "first-bucket"
    key    = "first.tfstate"
  }
}
`
				content2 := `
terraform {
  backend "s3" {
    bucket = "second-bucket"
    key    = "second.tfstate"
  }
}
`
				if err := os.WriteFile(filepath.Join(dir, "backend1.tf"), []byte(content1), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "backend2.tf"), []byte(content2), 0644)
			},
			want:    "s3://first-bucket/first.tfstate",
			wantErr: false,
		},
		{
			name: "S3 backend missing bucket",
			setupFunc: func(dir string) error {
				content := `
terraform {
  backend "s3" {
    key = "terraform.tfstate"
  }
}
`
				return os.WriteFile(filepath.Join(dir, "backend.tf"), []byte(content), 0644)
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "S3 backend missing key",
			setupFunc: func(dir string) error {
				content := `
terraform {
  backend "s3" {
    bucket = "my-bucket"
  }
}
`
				return os.WriteFile(filepath.Join(dir, "backend.tf"), []byte(content), 0644)
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "tfclean-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			app := &App{
				CLI: &CLI{
					Dir: tmpDir,
				},
			}

			got, err := app.detectBackendFromConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("detectBackendFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("detectBackendFromConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_buildS3URL(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name: "Valid S3 backend",
			content: `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "terraform.tfstate"
  }
}
`,
			want:    "s3://my-bucket/terraform.tfstate",
			wantErr: false,
		},
		{
			name: "S3 backend with path",
			content: `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "env/prod/terraform.tfstate"
  }
}
`,
			want:    "s3://my-bucket/env/prod/terraform.tfstate",
			wantErr: false,
		},
		{
			name: "S3 backend missing bucket",
			content: `
terraform {
  backend "s3" {
    key = "terraform.tfstate"
  }
}
`,
			want:    "",
			wantErr: true,
		},
		{
			name: "S3 backend missing key",
			content: `
terraform {
  backend "s3" {
    bucket = "my-bucket"
  }
}
`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "tfclean-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			filePath := filepath.Join(tmpDir, "backend.tf")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			parser := hclparse.NewParser()
			hclFile, diags := parser.ParseHCLFile(filePath)
			if diags.HasErrors() {
				t.Fatalf("Failed to parse HCL: %v", diags)
			}

			body, ok := hclFile.Body.(*hclsyntax.Body)
			if !ok {
				t.Fatalf("Failed to cast body")
			}

			var backendBlock *hclsyntax.Block
			for _, block := range body.Blocks {
				if block.Type == "terraform" {
					for _, b := range block.Body.Blocks {
						if b.Type == "backend" {
							backendBlock = b
							break
						}
					}
				}
			}

			if backendBlock == nil {
				t.Fatalf("Backend block not found")
			}

			app := &App{}
			got, err := app.buildS3URL(backendBlock)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildS3URL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("buildS3URL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_buildStateURLFromBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclean-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "terraform.tfstate"
  }
}
`

	filePath := filepath.Join(tmpDir, "backend.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := hclparse.NewParser()
	hclFile, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		t.Fatalf("Failed to parse HCL: %v", diags)
	}

	body, ok := hclFile.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatalf("Failed to cast body")
	}

	var backendBlock *hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type == "terraform" {
			for _, b := range block.Body.Blocks {
				if b.Type == "backend" {
					backendBlock = b
					break
				}
			}
		}
	}

	if backendBlock == nil {
		t.Fatalf("Backend block not found")
	}

	app := &App{}
	got, err := app.buildStateURLFromBackend(backendBlock)
	if err != nil {
		t.Errorf("buildStateURLFromBackend() error = %v", err)
		return
	}
	want := "s3://my-bucket/terraform.tfstate"
	if got != want {
		t.Errorf("buildStateURLFromBackend() = %v, want %v", got, want)
	}
}
