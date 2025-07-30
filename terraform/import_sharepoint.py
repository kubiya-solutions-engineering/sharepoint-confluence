#!/usr/bin/env python3
import sys
import json
import urllib.request
import urllib.error
import urllib.parse
import base64
import ssl
import re
import os

# Function to convert HTML to plain text with better formatting preservation
def html_to_text(html_content):
    # Process the content in stages to better preserve structure
    
    # 1. Handle special SharePoint elements
    html_content = re.sub(r'<div[^>]*class="[^"]*ms-rte[^"]*"[^>]*>.*?</div>', '', html_content, flags=re.DOTALL)  # Remove SharePoint RTE elements
    
    # 2. Handle headers
    html_content = re.sub(r'<h1[^>]*>(.*?)</h1>', r'\n\n# \1\n\n', html_content)
    html_content = re.sub(r'<h2[^>]*>(.*?)</h2>', r'\n\n## \1\n\n', html_content)
    html_content = re.sub(r'<h3[^>]*>(.*?)</h3>', r'\n\n### \1\n\n', html_content)
    html_content = re.sub(r'<h4[^>]*>(.*?)</h4>', r'\n\n#### \1\n\n', html_content)
    html_content = re.sub(r'<h5[^>]*>(.*?)</h5>', r'\n\n##### \1\n\n', html_content)
    html_content = re.sub(r'<h6[^>]*>(.*?)</h6>', r'\n\n###### \1\n\n', html_content)
    
    # 3. Handle lists - preserve nesting structure
    # Convert unordered lists
    html_content = re.sub(r'<ul[^>]*>', r'\n', html_content)
    html_content = re.sub(r'</ul>', r'\n', html_content)
    
    # Convert list items with proper indentation
    html_content = re.sub(r'<li[^>]*>(.*?)</li>', r'- \1\n', html_content)
    
    # Convert ordered lists
    html_content = re.sub(r'<ol[^>]*>', r'\n', html_content)
    html_content = re.sub(r'</ol>', r'\n', html_content)
    
    # 4. Handle paragraphs and line breaks
    html_content = re.sub(r'<p[^>]*>(.*?)</p>', r'\n\n\1\n\n', html_content)
    html_content = re.sub(r'<br[^>]*>', r'\n', html_content)
    html_content = re.sub(r'<div[^>]*>(.*?)</div>', r'\n\1\n', html_content)
    
    # 5. Handle text formatting
    html_content = re.sub(r'<strong[^>]*>(.*?)</strong>', r'**\1**', html_content)
    html_content = re.sub(r'<b[^>]*>(.*?)</b>', r'**\1**', html_content)
    html_content = re.sub(r'<em[^>]*>(.*?)</em>', r'*\1*', html_content)
    html_content = re.sub(r'<i[^>]*>(.*?)</i>', r'*\1*', html_content)
    html_content = re.sub(r'<u[^>]*>(.*?)</u>', r'_\1_', html_content)
    html_content = re.sub(r'<code[^>]*>(.*?)</code>', r'`\1`', html_content)
    html_content = re.sub(r'<pre[^>]*>(.*?)</pre>', r'```\n\1\n```', html_content, flags=re.DOTALL)
    
    # 6. Handle links
    html_content = re.sub(r'<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>', r'[\2](\1)', html_content)
    
    # 7. Handle tables (simplified conversion)
    html_content = re.sub(r'<table[^>]*>.*?</table>', r'\n[Table content omitted]\n', html_content, flags=re.DOTALL)
    
    # 8. Remove remaining HTML tags
    html_content = re.sub(r'<[^>]+>', ' ', html_content)
    
    # 9. Replace HTML entities
    entities = {
        '&nbsp;': ' ',
        '&lt;': '<',
        '&gt;': '>',
        '&amp;': '&',
        '&quot;': '"',
        '&apos;': "'",
        '&ldquo;': '"',
        '&rdquo;': '"',
        '&lsquo;': "'",
        '&rsquo;': "'",
        '&mdash;': '—',
        '&ndash;': '–',
        '&rarr;': '→',
        '&larr;': '←',
        '&uarr;': '↑',
        '&darr;': '↓',
        '&hellip;': '...',
    }
    
    for entity, replacement in entities.items():
        html_content = html_content.replace(entity, replacement)
    
    # 10. Clean up excessive whitespace while preserving paragraph breaks
    # Replace multiple newlines with just two (to create paragraph breaks)
    html_content = re.sub(r'\n{3,}', '\n\n', html_content)
    
    # Replace multiple spaces with a single space
    html_content = re.sub(r' +', ' ', html_content)
    
    # Trim leading/trailing whitespace
    html_content = html_content.strip()
    
    return html_content

# Function to get Azure AD access token
def get_access_token(tenant_id, client_id, client_secret, scope="https://graph.microsoft.com/.default"):
    try:
        # Azure AD token endpoint
        token_url = f"https://login.microsoftonline.com/{tenant_id}/oauth2/v2.0/token"
        
        # Prepare the request data
        data = {
            'grant_type': 'client_credentials',
            'client_id': client_id,
            'client_secret': client_secret,
            'scope': scope
        }
        
        # Encode the data
        data_encoded = urllib.parse.urlencode(data).encode('utf-8')
        
        # Create the request
        req = urllib.request.Request(token_url, data=data_encoded, method='POST')
        req.add_header('Content-Type', 'application/x-www-form-urlencoded')
        
        # Make the request
        context = ssl.create_default_context()
        response = urllib.request.urlopen(req, context=context, timeout=30)
        
        # Parse the response
        token_data = json.loads(response.read().decode('utf-8'))
        return token_data.get('access_token')
        
    except Exception as e:
        return None

# Simple function to make SharePoint API requests
def make_sharepoint_request(url, access_token):
    try:
        # Create request with headers
        req = urllib.request.Request(url)
        req.add_header('Authorization', f'Bearer {access_token}')
        req.add_header('Accept', 'application/json')
        
        # Make request with SSL context
        context = ssl.create_default_context()
        response = urllib.request.urlopen(req, context=context, timeout=30)
        
        # Read and decode response
        data = response.read().decode('utf-8')
        return json.loads(data)
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP Error: {e.code} - {e.reason}"}
    except urllib.error.URLError as e:
        return {"error": f"URL Error: {e.reason}"}
    except Exception as e:
        return {"error": f"Error: {str(e)}"}

# Function to recursively scan folders for files
def scan_drive_items(drive_id, folder_id, access_token, items_list, depth=0, max_depth=3):
    """Recursively scan a drive folder for files"""
    if depth > max_depth:
        print(f"DEBUG: Max depth {max_depth} reached, stopping recursion", file=sys.stderr)
        return
    
    # Get items from the folder
    items_url = f"https://graph.microsoft.com/v1.0/drives/{drive_id}/items/{folder_id}/children" if folder_id else f"https://graph.microsoft.com/v1.0/drives/{drive_id}/root/children"
    items_result = make_sharepoint_request(items_url, access_token)
    
    if "error" in items_result or "value" not in items_result:
        print(f"DEBUG: Error or no items in folder: {items_result}", file=sys.stderr)
        return
    
    print(f"DEBUG: Scanning folder (depth {depth}), found {len(items_result['value'])} items", file=sys.stderr)
    
    for item in items_result["value"]:
        if "file" in item:
            # It's a file
            file_name = item.get("name", "")
            file_extension = file_name.split('.')[-1].lower() if '.' in file_name else ""
            
            # For supported file types, add metadata as content
            if file_extension in ["txt", "md", "docx", "pdf", "xlsx", "pptx", "doc", "ppt"]:
                # Create content with file metadata and location info
                file_path = item.get("parentReference", {}).get("path", "").replace("/drives/" + drive_id + "/root:", "")
                content = f"Document: {file_name}\nType: {file_extension.upper()}\nSize: {item.get('size', 'Unknown')} bytes"
                if file_path:
                    content += f"\nLocation: {file_path}"
                
                # Add web URL if available
                if item.get("webUrl"):
                    content += f"\nURL: {item.get('webUrl')}"
                
                items_list.append({
                    "id": item.get("id", ""),
                    "title": file_name,
                    "content": content,
                    "type": "document",
                    "labels": f"sharepoint,document,{file_extension}"
                })
                print(f"DEBUG: Added document: {file_name} (from depth {depth})", file=sys.stderr)
        elif "folder" in item:
            # It's a folder - scan recursively
            folder_name = item.get("name", "")
            folder_id = item.get("id", "")
            print(f"DEBUG: Entering folder: {folder_name} (depth {depth})", file=sys.stderr)
            
            # Recurse into the folder
            scan_drive_items(drive_id, folder_id, access_token, items_list, depth + 1, max_depth)
            
            # Also add folder as a knowledge item with its contents summary
            child_count = item.get("folder", {}).get("childCount", 0)
            folder_content = f"SharePoint Folder: {folder_name}\nContains: {child_count} items"
            if item.get("webUrl"):
                folder_content += f"\nURL: {item.get('webUrl')}"
            
            items_list.append({
                "id": item.get("id", ""),
                "title": f"📁 {folder_name}",
                "content": folder_content,
                "type": "folder",
                "labels": "sharepoint,folder"
            })
            print(f"DEBUG: Added folder metadata: {folder_name}", file=sys.stderr)

def extract_site_info(site_url):
    """Extract hostname and site path from SharePoint URL"""
    try:
        # Remove protocol
        url_parts = site_url.replace('https://', '').replace('http://', '')
        
        # Split by /
        parts = url_parts.split('/')
        hostname = parts[0]
        
        # Get site path
        if len(parts) > 1:
            site_path = '/' + '/'.join(parts[1:])
        else:
            site_path = '/'
            
        return hostname, site_path
    except Exception:
        return None, None

def main():
    # Read input from stdin
    try:
        input_data = json.loads(sys.stdin.read())
    except json.JSONDecodeError:
        print(json.dumps({"error": "Failed to parse input JSON"}), file=sys.stderr)
        sys.exit(1)
    
    # Extract parameters
    site_url = input_data.get("SHAREPOINT_SITE_URL", "").rstrip('/')
    include_documents = input_data.get("include_documents", "true").lower() == "true"
    document_libraries = input_data.get("document_libraries", "Documents").split(',')
    
    # Get Azure credentials from input parameters (passed from Terraform)
    client_id = input_data.get("AZURE_CLIENT_ID", "")
    client_secret = input_data.get("AZURE_CLIENT_SECRET", "")
    tenant_id = input_data.get("AZURE_TENANT_ID", "")
    
    # Check for required parameters - if all are empty, SharePoint is disabled
    if not site_url and not client_id and not client_secret and not tenant_id:
        print(f"DEBUG: SharePoint is disabled - returning empty results", file=sys.stderr)
        print(json.dumps({"items": "[]"}))
        sys.exit(0)
    
    # Check for required parameters when SharePoint is enabled
    if not site_url or not client_id or not client_secret or not tenant_id:
        print(json.dumps({"error": "Missing required parameters. Ensure AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, and AZURE_TENANT_ID are set as environment variables."}), file=sys.stderr)
        sys.exit(1)
    
    print(f"DEBUG: Connecting to site: {site_url}", file=sys.stderr)
    
    # Get access token
    access_token = get_access_token(tenant_id, client_id, client_secret)
    if not access_token:
        print(json.dumps({"error": "Failed to get access token"}), file=sys.stderr)
        sys.exit(1)
    
    print("DEBUG: Successfully obtained access token", file=sys.stderr)
    
    # Extract site information
    hostname, site_path = extract_site_info(site_url)
    if not hostname:
        print(json.dumps({"error": "Invalid SharePoint site URL"}), file=sys.stderr)
        sys.exit(1)
    
    print(f"DEBUG: Extracted hostname: {hostname}, site_path: {site_path}", file=sys.stderr)
    
    # Get site ID using Microsoft Graph API
    graph_site_url = f"https://graph.microsoft.com/v1.0/sites/{hostname}:{site_path}"
    print(f"DEBUG: Requesting site info from: {graph_site_url}", file=sys.stderr)
    site_result = make_sharepoint_request(graph_site_url, access_token)
    
    if "error" in site_result:
        print(json.dumps({"error": f"SharePoint connection failed: {site_result['error']}"}), file=sys.stderr)
        sys.exit(1)
    
    site_id = site_result.get('id')
    if not site_id:
        print(json.dumps({"error": "Could not retrieve site ID"}), file=sys.stderr)
        sys.exit(1)
    
    print(f"DEBUG: Successfully retrieved site ID: {site_id}", file=sys.stderr)
    
    items = []
    
    # Get site pages
    pages_url = f"https://graph.microsoft.com/v1.0/sites/{site_id}/pages"
    print(f"DEBUG: Requesting pages from: {pages_url}", file=sys.stderr)
    pages_result = make_sharepoint_request(pages_url, access_token)
    
    print(f"DEBUG: Pages result: {pages_result}", file=sys.stderr)
    
    if "error" not in pages_result and "value" in pages_result:
        print(f"DEBUG: Found {len(pages_result['value'])} pages", file=sys.stderr)
        for page in pages_result["value"]:
            page_id = page.get("id")
            if page_id:
                # Get page content
                page_content_url = f"https://graph.microsoft.com/v1.0/sites/{site_id}/pages/{page_id}?$expand=webParts"
                page_data = make_sharepoint_request(page_content_url, access_token)
                
                if "error" not in page_data:
                    # Extract content from web parts
                    content_parts = []
                    if "webParts" in page_data and "value" in page_data["webParts"]:
                        for webpart in page_data["webParts"]["value"]:
                            if "innerHtml" in webpart:
                                content_parts.append(webpart["innerHtml"])
                    
                    # If no webParts content, use page description or title
                    content = " ".join(content_parts)
                    if not content and page_data.get("description"):
                        content = page_data.get("description", "")
                    
                    clean_content = html_to_text(content) if content else ""
                    
                    # If still no content, create a basic summary from page metadata
                    if not clean_content or clean_content.strip() == "":
                        title = page_data.get("title", "Untitled")
                        description = page_data.get("description", "")
                        clean_content = f"SharePoint Page: {title}"
                        if description:
                            clean_content += f"\n\nDescription: {description}"
                        clean_content += f"\n\nPage Layout: {page_data.get('pageLayout', 'Unknown')}"
                        
                        print(f"DEBUG: Creating basic content for page: {title}", file=sys.stderr)
                    
                    # Add to items
                    items.append({
                        "id": page_data.get("id", ""),
                        "title": page_data.get("title", "Untitled"),
                        "content": clean_content,
                        "type": "page",
                        "labels": "sharepoint,page"
                    })
                    print(f"DEBUG: Added page: {page_data.get('title', 'Untitled')}", file=sys.stderr)
    else:
        print(f"DEBUG: No pages found or error accessing pages: {pages_result}", file=sys.stderr)
    
    # Get documents from specified libraries if requested
    if include_documents:
        print(f"DEBUG: Checking document libraries: {document_libraries}", file=sys.stderr)
        for library_name in document_libraries:
            library_name = library_name.strip()
            if not library_name:
                continue
                
            # Get drive for the document library
            drives_url = f"https://graph.microsoft.com/v1.0/sites/{site_id}/drives"
            print(f"DEBUG: Requesting drives from: {drives_url}", file=sys.stderr)
            drives_result = make_sharepoint_request(drives_url, access_token)
            
            print(f"DEBUG: Drives result: {drives_result}", file=sys.stderr)
            
            if "error" not in drives_result and "value" in drives_result:
                print(f"DEBUG: Found {len(drives_result['value'])} drives", file=sys.stderr)
                for drive in drives_result["value"]:
                    print(f"DEBUG: Drive name: '{drive.get('name', '')}' (looking for '{library_name}')", file=sys.stderr)
                    
                target_drive_id = None
                for drive in drives_result["value"]:
                    if drive.get("name", "").lower() == library_name.lower():
                        target_drive_id = drive.get("id")
                        break
                
                if target_drive_id:
                    print(f"DEBUG: Found target drive ID: {target_drive_id}", file=sys.stderr)
                    # Recursively scan the drive for files and folders
                    print(f"DEBUG: Starting recursive scan of drive: {library_name}", file=sys.stderr)
                    scan_drive_items(target_drive_id, None, access_token, items)
                else:
                    print(f"DEBUG: Could not find drive for library: {library_name}", file=sys.stderr)
    
    print(f"DEBUG: Total items found: {len(items)}", file=sys.stderr)
    
    # Convert the items list to a JSON string
    items_json = json.dumps(items)
    
    # Return the items as a string value
    print(json.dumps({"items": items_json}))

if __name__ == "__main__":
    main() 