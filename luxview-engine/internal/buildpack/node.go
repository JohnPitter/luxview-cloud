package buildpack

import "fmt"

// npmInstallCmd returns a shell command that uses `npm ci` when a lockfile
// exists and falls back to `npm install` otherwise. This avoids build
// failures for repos that do not commit a package-lock.json.
func npmInstallCmd(flags string) string {
	cmd := "npm ci"
	fallback := "npm install"
	if flags != "" {
		cmd += " " + flags
		fallback += " " + flags
	}
	return fmt.Sprintf(`if [ -f package-lock.json ]; then %s; else %s; fi`, cmd, fallback)
}

// NextJsPack detects Next.js projects.
type NextJsPack struct{}

func (n *NextJsPack) Name() string { return "nextjs" }

func (n *NextJsPack) Detect(files []string) bool {
	s := fileSet(files)
	if !s["package.json"] {
		return false
	}
	return s["next.config.js"] || s["next.config.mjs"] || s["next.config.ts"]
}

func (n *NextJsPack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN %s
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE %d
CMD ["node", "server.js"]
`, npmInstallCmd(""), n.DefaultPort())
}

func (n *NextJsPack) DefaultPort() int { return 3000 }

// VitePack detects Vite-based projects (React, Vue, etc.) and builds to static.
type VitePack struct{}

func (v *VitePack) Name() string { return "vite" }

func (v *VitePack) Detect(files []string) bool {
	s := fileSet(files)
	if !s["package.json"] {
		return false
	}
	return s["vite.config.js"] || s["vite.config.ts"] || s["vite.config.mjs"]
}

func (v *VitePack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN %s
COPY . .
RUN npx vite build --base=/

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
RUN printf 'server {\n    listen %d;\n    root /usr/share/nginx/html;\n    index index.html;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
EXPOSE %d
CMD ["nginx", "-g", "daemon off;"]
`, npmInstallCmd(""), v.DefaultPort(), v.DefaultPort())
}

func (v *VitePack) DefaultPort() int { return 80 }

// NodePack detects generic Node.js projects.
type NodePack struct{}

func (n *NodePack) Name() string { return "node" }

func (n *NodePack) Detect(files []string) bool {
	s := fileSet(files)
	return s["package.json"]
}

func (n *NodePack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN %s
COPY . .
EXPOSE %d
CMD ["npm", "start"]
`, npmInstallCmd("--omit=dev"), n.DefaultPort())
}

func (n *NodePack) DefaultPort() int { return 3000 }
