# ====================================
# Core Configuration Variables
# ====================================

variable "teammate_name" {
  description = "Name of your Knowledge Assistant teammate"
  type        = string
  default     = "knowledge-assistant"
}

variable "kubiya_runner" {
  description = "Kubiya runner to use for the agent"
  type        = string
}

variable "kubiya_user_email" {
  description = "Email of the Kubiya user who will own the imported knowledge items"
  type        = string
}

variable "kubiya_groups_allowed_groups" {
  description = "Groups allowed to interact with the teammate"
  type        = list(string)
  default     = ["Admin", "Users"]
}

variable "debug_mode" {
  description = "Debug mode - set to true to see detailed outputs during runtime"
  type        = bool
  default     = false
}

# ====================================
# SharePoint Configuration Variables
# ====================================

variable "sharepoint_site_url" {
  description = "The URL of your SharePoint site"
  type        = string
  default     = ""
}

variable "azure_client_id" {
  description = "Azure AD Client ID for SharePoint access"
  type        = string
  default     = ""
}

variable "azure_client_secret" {
  description = "Azure AD Client Secret for SharePoint access"
  type        = string
  sensitive   = true
  default     = ""
}

variable "azure_tenant_id" {
  description = "Azure AD Tenant ID for SharePoint access"
  type        = string
  default     = ""
}

variable "sharepoint_document_libraries" {
  description = "List of document library names to import content from"
  type        = list(string)
  default     = ["Documents", "Shared Documents"]
}

variable "import_sharepoint_documents" {
  description = "Whether to import documents from SharePoint document libraries"
  type        = bool
  default     = true
}

variable "enable_sharepoint" {
  description = "Whether to enable SharePoint knowledge import"
  type        = bool
  default     = false
}

# ====================================
# Confluence Configuration Variables
# ====================================

variable "confluence_url" {
  description = "URL of your Confluence instance"
  type        = string
  default     = ""
}

variable "confluence_username" {
  description = "Username or email for Confluence authentication"
  type        = string
  default     = ""
}

variable "CONFLUENCE_API_TOKEN" {
  description = "API token for Confluence authentication"
  type        = string
  sensitive   = true
  default     = ""
}

variable "confluence_space_keys" {
  description = "List of Confluence space keys to import. For single space: ['DSO'], for multiple: ['DSO', 'DOCS', 'TECH']"
  type        = list(string)
  default     = []
}

variable "import_confluence_blogs" {
  description = "Whether to import blog posts from Confluence"
  type        = bool
  default     = true
}

variable "max_pages" {
  description = "Maximum number of pages to import from Confluence (distributed across all spaces if multiple)"
  type        = number
  default     = 10000
}

variable "enable_confluence" {
  description = "Whether to enable Confluence knowledge import"
  type        = bool
  default     = false
} 