package buildpack

import "fmt"

// StaticPack detects plain static HTML/CSS/JS sites.
type StaticPack struct{}

func (s *StaticPack) Name() string { return "static" }

func (s *StaticPack) Detect(files []string) bool {
	set := fileSet(files)
	return set["index.html"]
}

func (s *StaticPack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM nginx:alpine
COPY . /usr/share/nginx/html
COPY <<'CONF' /etc/nginx/conf.d/default.conf
server {
    listen %d;
    root /usr/share/nginx/html;
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
CONF
EXPOSE %d
CMD ["nginx", "-g", "daemon off;"]
`, s.DefaultPort(), s.DefaultPort())
}

func (s *StaticPack) DefaultPort() int { return 80 }
