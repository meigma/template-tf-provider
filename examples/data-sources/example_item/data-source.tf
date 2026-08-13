data "example_item" "web_frontend" {
  name = "web-frontend"
}

output "web_frontend_id" {
  value = data.example_item.web_frontend.id
}
