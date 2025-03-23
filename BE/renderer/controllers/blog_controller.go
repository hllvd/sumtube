// controllers/blog_controller.go
package controllers

import (
	"net/http"
	"strings"

	"go-renderer-server/services"
	"go-renderer-server/templates"

	"github.com/a-h/templ"
)

// BlogController handles requests for the blog template.
type BlogController struct{}

// NewBlogController creates a new BlogController.
func NewBlogController() *BlogController {
	return &BlogController{}
}

// HandleBlog handles the /ID route.
func (c *BlogController) HandleBlog(w http.ResponseWriter, r *http.Request) {
	// Extract the ID from the URL path
	id := strings.TrimPrefix(r.URL.Path, "/")

	// Call the external API to get blog data
	blogData, err := services.GetBlogData(id)
	if err != nil {
		http.Error(w, "Failed to fetch blog data", http.StatusInternalServerError)
		return
	}

	// Render the blog template with the blog data
	templ.Handler(templates.BlogTemplate(blogData)).ServeHTTP(w, r)
}