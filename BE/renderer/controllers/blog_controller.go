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

func (c *BlogController) HandleBlog(w http.ResponseWriter, r *http.Request) {
	println("HandleBlog")
	// Extract the path from the URL
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Split the path into parts
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// Extract lang, title, and id
	lang := parts[0]
	titleWithID := parts[1]

	// Split the title and ID
	titleIDParts := strings.Split(titleWithID, "~")
	if len(titleIDParts) != 2 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// title := titleIDParts[0]
	id := titleIDParts[1]

	// Now you have lang, title, and id
	// You can use them as needed, for example:
	// log.Printf("Language: %s, Title: %s, ID: %s", lang, title, id)

	// Call the external API to get blog data
	blogData, err := services.GetBlogData(lang, id)
	if err != nil {
		http.Error(w, "Failed to fetch blog data", http.StatusInternalServerError)
		return
	}

	// Render the blog template with the blog data
	templ.Handler(templates.BlogTemplate(blogData)).ServeHTTP(w, r)
}