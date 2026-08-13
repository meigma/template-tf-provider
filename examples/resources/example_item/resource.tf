resource "example_item" "web_frontend" {
  name        = "web-frontend"
  description = "Public entry point for the web tier"
  tags        = ["edge", "prod"]
}
