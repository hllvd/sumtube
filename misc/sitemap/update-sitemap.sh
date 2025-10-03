#!/bin/bash
set -e  # exit on first error

# 1. Generate sitemaps locally
go run main.go copy-pt && \
go run main.go copy-en && \
go run main.go copy-es && \
go run main.go copy-fr && \
go run main.go copy-it

# 2. Add ONLY sitemap files
echo "🔄 Adding sitemap files..."
git add /Users/hudsonvandal/Documents/sumtube/BE/renderer/static/sitemaps/*.xml

# 3. Commit
if git diff --cached --quiet; then
  echo "✅ No changes to commit."
else
  git commit -m "sitemap generator script"
  echo "✅ Changes committed."
fi

# 4. Push (so remote can pull latest)
git push

# 5. Connect to server and pull
ssh sumtube-ec2 << 'EOF'
  cd ~/sumtube/BE || exit 1
  echo "🔄 Pulling latest changes..."
  git pull
  echo "✅ Done!"
EOF
