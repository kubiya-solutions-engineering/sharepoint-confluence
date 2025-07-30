# Combined Knowledge Assistant - SharePoint + Confluence

This Terraform module creates a unified Kubiya Knowledge Assistant that can import and provide access to content from both SharePoint sites and Confluence spaces.

## Features

- **Dual Source Support**: Import knowledge from both SharePoint and Confluence
- **Selective Import**: Enable/disable each source independently
- **Comprehensive Content**: Supports SharePoint pages, documents, Confluence pages, and blog posts
- **Multi-Space/Multi-Library**: Handle multiple Confluence spaces and SharePoint document libraries
- **Intelligent Processing**: HTML-to-text conversion, content validation, and size optimization
- **Slack Integration**: Built-in Slack integration for easy team access

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   SharePoint    │    │                  │    │   Confluence    │
│     Site        │────┤  Knowledge       │────┤     Spaces      │
│                 │    │   Assistant      │    │                 │
│ • Pages         │    │                  │    │ • Pages         │
│ • Documents     │    │                  │    │ • Blog Posts    │
│ • Libraries     │    │                  │    │ • Attachments   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────┐
                       │    Slack    │
                       │ Integration │
                       └─────────────┘
```

## Quick Start

1. **Configure your sources** in `terraform.tfvars`:
   ```hcl
   # Enable the sources you want
   enable_sharepoint = true
   enable_confluence = true
   
   # Configure SharePoint (if enabled)
   sharepoint_site_url = "https://yourcompany.sharepoint.com/sites/YourSite"
   azure_client_id = "your-azure-client-id"
   azure_client_secret = "your-azure-client-secret"
   azure_tenant_id = "your-azure-tenant-id"
   
   # Configure Confluence (if enabled)
   confluence_url = "https://yourcompany.atlassian.net/wiki"
   confluence_username = "your-confluence-username"
   CONFLUENCE_API_TOKEN = "your-confluence-api-token"
   confluence_space_keys = ["SPACE1", "SPACE2"]  # Single: ["DSO"] or Multiple: ["DSO", "DOCS"]
   ```

2. **Deploy the solution**:
   ```bash
   terraform init
   terraform apply -var-file="terraform.tfvars"
   ```

3. **Access your assistant** via Slack using the configured teammate name.

## Configuration Guide

### Core Settings

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `teammate_name` | Name of your Knowledge Assistant | Yes | `"knowledge-assistant"` |
| `kubiya_runner` | Kubiya runner to use | Yes | - |
| `kubiya_user_email` | Email of the Kubiya user | Yes | - |
| `kubiya_groups_allowed_groups` | Allowed user groups | No | `["Admin", "Users"]` |
| `debug_mode` | Enable debug logging | No | `false` |

### SharePoint Configuration

| Variable | Description | Required (if enabled) | Default |
|----------|-------------|----------------------|---------|
| `enable_sharepoint` | Enable SharePoint import | No | `false` |
| `sharepoint_site_url` | SharePoint site URL | Yes | `""` |
| `azure_client_id` | Azure AD Client ID | Yes | `""` |
| `azure_client_secret` | Azure AD Client Secret | Yes | `""` |
| `azure_tenant_id` | Azure AD Tenant ID | Yes | `""` |
| `sharepoint_document_libraries` | Document libraries to import | No | `["Documents", "Shared Documents"]` |
| `import_sharepoint_documents` | Import documents | No | `true` |

### Confluence Configuration

| Variable | Description | Required (if enabled) | Default |
|----------|-------------|----------------------|---------|
| `enable_confluence` | Enable Confluence import | No | `false` |
| `confluence_url` | Confluence instance URL | Yes | `""` |
| `confluence_username` | Confluence username | Yes | `""` |
| `CONFLUENCE_API_TOKEN` | Confluence API token | Yes | `""` |
| `confluence_space_keys` | List of space keys (single or multiple) | Yes | `[]` |
| `import_confluence_blogs` | Import blog posts | No | `true` |
| `max_pages` | Max pages to import | No | `10000` |

## Usage Scenarios

### 1. SharePoint Only
```hcl
enable_sharepoint = true
enable_confluence = false

sharepoint_site_url = "https://company.sharepoint.com/sites/docs"
# ... other SharePoint settings
```

### 2. Confluence Only
```hcl
enable_sharepoint = false
enable_confluence = true

confluence_url = "https://company.atlassian.net/wiki"
confluence_space_keys = ["DOCS", "POLICIES"]  # Single space: ["DOCS"] or multiple spaces
# ... other Confluence settings
```

### 3. Both Sources (Recommended)
```hcl
enable_sharepoint = true
enable_confluence = true

# Configure both SharePoint and Confluence settings
# ... all required settings for both sources
```

### 4. Selective Content Import
```hcl
# SharePoint: Only pages, no documents
import_sharepoint_documents = false

# Confluence: Only pages, no blog posts
import_confluence_blogs = false

# Single Confluence space: confluence_space_keys = ["DSO"]
# Multiple spaces: confluence_space_keys = ["DSO", "DOCS", "POLICIES"]

# Limit Confluence pages
max_pages = 1000  # Distributed across all spaces
```

## Content Processing

### SharePoint Content
- **Pages**: HTML content converted to markdown-like text
- **Documents**: Metadata and basic information (file size, type, location)
- **Folders**: Directory structure and summary information
- **Libraries**: Support for multiple document libraries

### Confluence Content
- **Pages**: Full HTML content converted to clean text with formatting preservation
- **Blog Posts**: Same processing as pages with blog-specific labeling
- **Tables**: Converted to markdown table format
- **Links**: Preserved with markdown link syntax
- **Code Blocks**: Properly formatted code sections

## Authentication Setup

### SharePoint (Azure AD)
1. Create an Azure AD application
2. Grant necessary SharePoint permissions:
   - `Sites.Read.All`
   - `Files.Read.All`
3. Generate client secret
4. Note down: Client ID, Client Secret, Tenant ID

### Confluence (API Token)
1. Go to [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Create a new API token
3. Use your Confluence email as username
4. Use the generated token as password

## File Structure

```
combined-knowledge-import/
├── main.tf                    # Main Terraform configuration
├── variables.tf               # Variable definitions
├── terraform.tfvars          # Configuration values
├── import_sharepoint.py       # SharePoint import script
├── import_confluence.go       # Confluence import script
├── build.sh                   # Go binary build script
└── README.md                  # This file
```

## Deployment Commands

```bash
# Initialize Terraform
terraform init

# Plan the deployment (review changes)
terraform plan -var-file="terraform.tfvars"

# Apply the configuration
terraform apply -var-file="terraform.tfvars"

# View outputs
terraform output

# Destroy when no longer needed
terraform destroy -var-file="terraform.tfvars"
```

## Troubleshooting

### Common Issues

1. **Go Binary Build Fails**
   ```bash
   # Manually build the binary
   chmod +x build.sh
   ./build.sh
   ```

2. **SharePoint Authentication Errors**
   - Verify Azure AD application permissions
   - Check client secret expiration
   - Ensure site URL is correct

3. **Confluence Connection Issues**
   - Verify API token is valid
   - Check username (should be email)
   - Ensure space keys exist and are accessible

4. **No Content Imported**
   - Enable debug mode: `debug_mode = true`
   - Check Terraform output for error messages
   - Verify user permissions on content

### Debug Mode

Enable detailed logging by setting `debug_mode = true` in your `terraform.tfvars`. This will:
- Show detailed import progress
- Display content processing statistics
- Include error messages and warnings
- Provide connection test results

## Advanced Configuration

### Multiple SharePoint Sites
Currently supports one site per deployment. For multiple sites, deploy separate instances with different `teammate_name` values.

### Large Content Handling
The system automatically:
- Limits content size to prevent embedding issues
- Validates content quality before import
- Handles pagination for large Confluence spaces
- Distributes page limits across multiple spaces

### Custom Labels and Organization
Content is automatically labeled with:
- Source system (`sharepoint`, `confluence`)
- Content type (`page`, `document`, `blog`, `folder`)
- Source identifier (`space-SPACENAME`, `site`)
- Custom labels from original content

## Support and Updates

For updates to the import scripts:
1. **Go Binary**: Modify `import_confluence.go` and run `./build.sh`
2. **Python Script**: Modify `import_sharepoint.py` directly
3. **Configuration**: Update `terraform.tfvars` and run `terraform apply`

## Security Considerations

- Store API tokens and secrets securely
- Use environment variables for sensitive data
- Regularly rotate API tokens
- Review user group permissions
- Monitor access logs in Kubiya dashboard 