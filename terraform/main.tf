terraform {
  required_providers {
    kubiya = {
      source = "kubiya-terraform/kubiya"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 2.0"
    }
    http = {
      source = "hashicorp/http"
    }
    local = {
      source = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

provider "kubiya" {
  // API key is set as an environment variable KUBIYA_API_KEY
}

# Update the tooling source to use Slack integration
resource "kubiya_source" "slack_tooling" {
  url = "https://github.com/kubiyabot/community-tools/tree/main/slack"
}

# Create local variables for Confluence space handling
locals {
  # Use confluence_space_keys directly - supports both single and multiple spaces
  confluence_spaces = var.enable_confluence ? var.confluence_space_keys : []
  
  # Create a space list string for the Go binary
  space_keys_string = join(",", local.confluence_spaces)
  
  # Determine knowledge sources description
  knowledge_sources = compact([
    var.enable_sharepoint ? "SharePoint" : "",
    var.enable_confluence ? "Confluence" : ""
  ])
  knowledge_sources_text = length(local.knowledge_sources) > 0 ? join(" and ", local.knowledge_sources) : "configured knowledge sources"
}

# Configure the Combined Knowledge Assistant agent
resource "kubiya_agent" "knowledge_assistant" {
  name         = var.teammate_name
  runner       = var.kubiya_runner
  description  = "AI-powered assistant that answers user queries using knowledge imported from ${local.knowledge_sources_text}"
  instructions = <<-EOT
Your primary role is to assist users by answering their questions using the knowledge sources attached to you.

You have access to knowledge from the following sources:
${var.enable_sharepoint ? "- SharePoint site: ${var.sharepoint_site_url}" : ""}
${var.enable_confluence ? "- Confluence spaces: ${local.space_keys_string}" : ""}

When responding to user queries:
1. Search through your knowledge sources to find relevant information.
2. Provide clear, concise answers based on the information you find.
3. Include context and references to the original ${local.knowledge_sources_text} pages and documents when possible.
4. If you can't find relevant information in your knowledge sources, clearly communicate this to the user.
5. When referencing content, mention which system it came from (SharePoint or Confluence) for better context.

Your goal is to be a helpful bridge between users and the knowledge contained within the ${local.knowledge_sources_text} documentation that has been imported as knowledge sources.
EOT
  sources      = [kubiya_source.slack_tooling.name]
  
  integrations = ["slack"]

  users  = []
  groups = var.kubiya_groups_allowed_groups

  environment_variables = {
    KUBIYA_TOOL_TIMEOUT = "500"
  }
  
  secrets = compact([
    var.enable_sharepoint ? "AZURE_CLIENT_ID" : "",
    var.enable_sharepoint ? "AZURE_TENANT_ID" : "",
    var.enable_sharepoint ? "AZURE_CLIENT_SECRET" : "",
    var.enable_confluence ? "CONFLUENCE_API_TOKEN" : ""
  ])
  
  is_debug_mode = var.debug_mode

  lifecycle {
    create_before_destroy = true
    ignore_changes       = []
  }
}

# Create Confluence API token secret (always create since we always call confluence data source)
resource "kubiya_secret" "confluence_api_token" {
  name        = "CONFLUENCE_API_TOKEN"
  value       = var.confluence_api_token != "" ? var.confluence_api_token : var.CONFLUENCE_API_TOKEN
  description = "Confluence API token for the Knowledge Assistant"

  lifecycle {
    create_before_destroy = true
    ignore_changes       = []
  }
}

# Build the Go binary automatically during terraform apply (always build since we always call confluence data source)
resource "null_resource" "build_go_binary" {
  triggers = {
    # Rebuild when Go source code changes
    go_source_hash = filesha256("${path.module}/import_confluence.go")
    # Also rebuild when build script changes
    build_script_hash = filesha256("${path.module}/build.sh")
  }

  provisioner "local-exec" {
    command = "${path.module}/build.sh"
    working_dir = path.module
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Fetch SharePoint content using data source (always fetch, handle enabling in locals)
data "external" "sharepoint_content" {
  program = ["python3", "${path.module}/import_sharepoint.py"]

  # Set parameters for the Python script
  query = {
    SHAREPOINT_SITE_URL = var.enable_sharepoint ? var.sharepoint_site_url : ""
    AZURE_CLIENT_ID = var.enable_sharepoint ? var.azure_client_id : ""
    AZURE_CLIENT_SECRET = var.enable_sharepoint ? var.azure_client_secret : ""
    AZURE_TENANT_ID = var.enable_sharepoint ? var.azure_tenant_id : ""
    include_documents = var.enable_sharepoint && var.import_sharepoint_documents ? "true" : "false"
    document_libraries = var.enable_sharepoint ? join(",", var.sharepoint_document_libraries) : ""
  }
}

# Fetch Confluence content using data source (always fetch, handle enabling in locals)
data "external" "confluence_content" {
  program = ["${path.module}/import_confluence"]

  # Set parameters for the Go binary (no conditional expressions - let script handle empty values)
  query = {
    CONFLUENCE_URL = var.confluence_url
    CONFLUENCE_USERNAME = var.confluence_username
    CONFLUENCE_API_TOKEN = var.confluence_api_token != "" ? var.confluence_api_token : var.CONFLUENCE_API_TOKEN
    space_keys = local.space_keys_string
    include_blogs = var.import_confluence_blogs ? "true" : "false"
    max_pages = var.max_pages
  }
}

# Add a null resource to handle retries for Confluence data source
resource "null_resource" "confluence_content_retry" {
  triggers = {
    confluence_url      = var.confluence_url
    confluence_username = var.confluence_username
    space_keys         = local.space_keys_string
    include_blogs      = var.import_confluence_blogs
    # Force recreation if the external data fails
    content_hash = try(substr(sha256(data.external.confluence_content.result.items), 0, 16), "failed")
  }

  provisioner "local-exec" {
    command = "echo 'Confluence content fetched successfully from spaces: ${local.space_keys_string}'"
  }

  # Add explicit dependencies
  depends_on = [data.external.confluence_content]
}

# Pre-filter and validate content for better debugging
locals {
  # Parse content directly from external sources (always parse, scripts handle disabled state)
  sharepoint_items = jsondecode(data.external.sharepoint_content.result.items)
  raw_confluence_items = jsondecode(data.external.confluence_content.result.items)
  
  # Create item maps for knowledge resources (exactly like goversionv2)
  sharepoint_knowledge_items = {
    for item in local.sharepoint_items : 
    item.id => item
    if try(
      item.content != null &&
      item.content != "" &&
      length(item.content) > 10,
      false
    )
  }
  
  # Filter valid items with enhanced validation (simplified) - matching goversionv2 exactly
  valid_confluence_items = {
    for item in local.raw_confluence_items : 
    item.id => item 
    if try(
      item.content != null && 
      item.content != "" && 
      length(item.content) > 30 &&          # Minimum meaningful content
      length(item.content) < 250000 &&       # Reduced max size to prevent embedding issues
      length(item.title) > 2 &&             # Valid title
      length(item.title) < 500 &&           # Reasonable title length
      item.space_key != null &&             # Must have space key
      item.space_key != "",                 # Space key cannot be empty
      false
    )
  }
}

# Create knowledge items for SharePoint content
resource "kubiya_knowledge" "sharepoint_content" {
  for_each = local.sharepoint_knowledge_items

  name             = each.value.title
  groups           = var.kubiya_groups_allowed_groups
  description      = "Imported from SharePoint site: ${var.sharepoint_site_url}"
  labels           = concat(
    ["sharepoint", "site"],
    each.value.type == "document" ? ["document"] : [],
    each.value.type == "page" ? ["page"] : [],
    split(",", each.value.labels)
  )
  supported_agents = [kubiya_agent.knowledge_assistant.name]
  
  # Use the content directly from the Python script
  content = each.value.content

  lifecycle {
    create_before_destroy = true
    ignore_changes       = []
  }

  # Add explicit dependencies
  depends_on = [
    kubiya_agent.knowledge_assistant
  ]
}

# Create knowledge items for Confluence content
resource "kubiya_knowledge" "confluence_content" {
  for_each = local.valid_confluence_items

  name             = substr(replace(each.value.title, "[^a-zA-Z0-9 -]", ""), 0, 80)  # Sanitize and limit title
  groups           = var.kubiya_groups_allowed_groups
  description      = "Imported from Confluence space: ${each.value.space_key}"
  labels           = concat(
    ["confluence", "space-${each.value.space_key}"],
    each.value.type == "blog" ? ["blog"] : [],
    length(split(",", each.value.labels)) > 0 && each.value.labels != "" ? split(",", each.value.labels) : []
  )
  supported_agents = [kubiya_agent.knowledge_assistant.name]
  
  # Clean and sanitize content
  content = each.value.content

  lifecycle {
    create_before_destroy = true
    ignore_changes       = []
    prevent_destroy = false
  }

  # Add explicit dependencies
  depends_on = [
    kubiya_agent.knowledge_assistant,
    null_resource.confluence_content_retry
  ]
}

# Output the agent details and statistics
output "knowledge_assistant" {
  sensitive = true
  value = {
    name       = kubiya_agent.knowledge_assistant.name
    debug_mode = var.debug_mode
    
    # SharePoint statistics
    sharepoint_enabled = var.enable_sharepoint
    sharepoint_site = var.enable_sharepoint ? var.sharepoint_site_url : "disabled"
    sharepoint_items_imported = var.enable_sharepoint ? length(kubiya_knowledge.sharepoint_content) : 0
    
    # Confluence statistics
    confluence_enabled = var.enable_confluence
    confluence_spaces_processed = var.enable_confluence ? local.confluence_spaces : []
    confluence_raw_items_fetched = var.enable_confluence ? length(local.raw_confluence_items) : 0
    confluence_valid_items_processed = var.enable_confluence ? length(local.valid_confluence_items) : 0
    confluence_knowledge_sources_created = var.enable_confluence ? length(kubiya_knowledge.confluence_content) : 0
    
    # Combined statistics
    total_knowledge_sources = length(kubiya_knowledge.sharepoint_content) + length(kubiya_knowledge.confluence_content)
    knowledge_sources_enabled = local.knowledge_sources
  }
} 