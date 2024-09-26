#!/bin/sh

# Generate the env-config.js file with the API_URL
cat <<EOF > /usr/share/nginx/html/env-config.js
window.__ENV__ = {
  API_URL: "${API_URL:-http://localhost:8080}"
};
EOF

nginx -g 'daemon off;' -c /etc/nginx/nginx.conf