# Stage 1: Build the SolidJS project
FROM node:alpine AS builder

WORKDIR /app

# Copy package.json and package-lock.json to leverage Docker cache
COPY package*.json ./

# Install dependencies
# --legacy-peer-deps added for vis-network
RUN npm install --legacy-peer-deps

# Copy the rest of the application code
COPY . .

# Build the SolidJS project
RUN npm run build

# Stage 2: Serve the built files using NGINX
FROM nginx:alpine

# Update with the nginx.conf in repo to allow 8080 to be used
# Default is only for http traffic
COPY --from=builder /app/nginx.conf /etc/nginx/nginx.conf

# Set the working directory to NGINX HTML directory
WORKDIR /usr/share/nginx/html

# Copy built files from the previous stage
COPY --from=builder /app/dist .

COPY entrypoint.sh .

RUN chmod +x entrypoint.sh

# Start NGINX in the foreground and listen on port 8080
CMD ["sh", "-c", "./entrypoint.sh"]