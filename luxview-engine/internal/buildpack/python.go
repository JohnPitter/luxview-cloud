package buildpack

import "fmt"

// PythonPack detects Python projects.
type PythonPack struct{}

func (p *PythonPack) Name() string { return "python" }

func (p *PythonPack) Detect(files []string) bool {
	s := fileSet(files)
	return s["requirements.txt"] || s["pyproject.toml"] || s["Pipfile"]
}

func (p *PythonPack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt* pyproject.toml* Pipfile* ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
    elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; \
    elif [ -f Pipfile ]; then pip install pipenv && pipenv install --deploy --system; fi
COPY . .
EXPOSE %d
CMD ["gunicorn", "--bind", "0.0.0.0:%d", "--workers", "2", "app:app"]
`, p.DefaultPort(), p.DefaultPort())
}

func (p *PythonPack) DefaultPort() int { return 8000 }
