package service

import (
	"fmt"
	"strings"

	"github.com/luxview/engine/internal/model"
)

const (
	defaultActionImage = "node:22-alpine"
	goActionImage      = "golang:1.23-alpine"
	javaActionImage    = "maven:3.9-eclipse-temurin-21"
	nodeActionImage    = "node:22-alpine"

	actionKindRun  = "run"
	actionKindUses = "uses"
)

type ParsedWorkflow struct {
	Name string
	Jobs []ParsedJob
}

type ParsedJob struct {
	Name  string
	Image string
	Steps []model.ActionStep
}

func ParseGitHubWorkflow(content string) (*ParsedWorkflow, error) {
	lines := strings.Split(content, "\n")
	workflow := &ParsedWorkflow{Name: "workflow"}
	var currentJob *ParsedJob
	inJobs := false
	inSteps := false
	position := 0

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := leadingSpaces(raw)

		if indent == 0 && strings.HasPrefix(line, "name:") {
			workflow.Name = cleanYAMLValue(strings.TrimPrefix(line, "name:"))
			continue
		}
		if indent == 0 && line == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if indent == 2 && strings.HasSuffix(line, ":") {
			if currentJob != nil {
				workflow.Jobs = append(workflow.Jobs, *currentJob)
			}
			jobName := strings.TrimSuffix(line, ":")
			currentJob = &ParsedJob{Name: jobName, Image: defaultActionImage}
			inSteps = false
			position = 0
			continue
		}
		if currentJob == nil {
			continue
		}
		if strings.HasPrefix(line, "runs-on:") {
			continue
		}
		if strings.HasPrefix(line, "steps:") {
			inSteps = true
			continue
		}
		if !inSteps {
			continue
		}
		if strings.HasPrefix(line, "- uses:") {
			position++
			uses := cleanYAMLValue(strings.TrimPrefix(line, "- uses:"))
			currentJob.Image = imageForAction(currentJob.Image, uses)
			currentJob.Steps = append(currentJob.Steps, model.ActionStep{
				Name:     uses,
				Kind:     actionKindUses,
				Uses:     uses,
				Status:   model.ActionQueued,
				Position: position,
			})
			continue
		}
		if strings.HasPrefix(line, "- run:") {
			position++
			command, nextIndex := parseRunValue(lines, i, strings.TrimPrefix(line, "- run:"))
			i = nextIndex
			currentJob.Steps = append(currentJob.Steps, model.ActionStep{
				Name:     command,
				Kind:     actionKindRun,
				Command:  command,
				Status:   model.ActionQueued,
				Position: position,
			})
			continue
		}
		if strings.HasPrefix(line, "- name:") {
			position++
			name := cleanYAMLValue(strings.TrimPrefix(line, "- name:"))
			currentJob.Steps = append(currentJob.Steps, model.ActionStep{
				Name:     name,
				Kind:     actionKindUses,
				Status:   model.ActionQueued,
				Position: position,
			})
			continue
		}
		if len(currentJob.Steps) == 0 {
			continue
		}
		last := &currentJob.Steps[len(currentJob.Steps)-1]
		if strings.HasPrefix(line, "uses:") {
			uses := cleanYAMLValue(strings.TrimPrefix(line, "uses:"))
			last.Kind = actionKindUses
			last.Uses = uses
			if last.Name == "" {
				last.Name = uses
			}
			currentJob.Image = imageForAction(currentJob.Image, uses)
		}
		if strings.HasPrefix(line, "run:") {
			command, nextIndex := parseRunValue(lines, i, strings.TrimPrefix(line, "run:"))
			i = nextIndex
			last.Kind = actionKindRun
			last.Command = command
			if last.Name == "" {
				last.Name = command
			}
		}
		if strings.HasPrefix(line, "with:") {
			inputs, nextIndex := parseWithBlock(lines, i)
			i = nextIndex
			last.Inputs = inputs
		}
	}
	if currentJob != nil {
		workflow.Jobs = append(workflow.Jobs, *currentJob)
	}
	if len(workflow.Jobs) == 0 {
		return nil, fmt.Errorf("workflow has no jobs")
	}
	return workflow, nil
}

func imageForAction(currentImage, uses string) string {
	switch {
	case strings.HasPrefix(uses, "actions/setup-go@"):
		return goActionImage
	case strings.HasPrefix(uses, "actions/setup-java@"):
		return javaActionImage
	case strings.HasPrefix(uses, "actions/setup-node@"):
		return nodeActionImage
	default:
		return currentImage
	}
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func cleanYAMLValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	return v
}

func parseRunValue(lines []string, index int, value string) (string, int) {
	value = cleanYAMLValue(value)
	if value != "|" && value != ">" {
		return value, index
	}

	baseIndent := leadingSpaces(lines[index])
	var parts []string
	nextIndex := index
	for i := index + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			parts = append(parts, "")
			nextIndex = i
			continue
		}
		if leadingSpaces(line) <= baseIndent {
			break
		}
		parts = append(parts, strings.TrimSpace(line))
		nextIndex = i
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n"), nextIndex
}

func parseWithBlock(lines []string, index int) (map[string]string, int) {
	baseIndent := leadingSpaces(lines[index])
	inputs := make(map[string]string)
	nextIndex := index

	for i := index + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextIndex = i
			continue
		}
		if leadingSpaces(line) <= baseIndent {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			nextIndex = i
			continue
		}
		cleanKey := cleanYAMLValue(key)
		cleanValue := cleanYAMLValue(value)
		if cleanValue == "|" || cleanValue == ">" {
			var block []string
			for j := i + 1; j < len(lines); j++ {
				childLine := lines[j]
				childTrimmed := strings.TrimSpace(childLine)
				if childTrimmed == "" {
					block = append(block, "")
					nextIndex = j
					continue
				}
				if leadingSpaces(childLine) <= leadingSpaces(line) {
					break
				}
				block = append(block, childTrimmed)
				nextIndex = j
			}
			inputs[cleanKey] = strings.TrimRight(strings.Join(block, "\n"), "\n")
			i = nextIndex
			continue
		}
		inputs[cleanKey] = cleanValue
		nextIndex = i
	}

	return inputs, nextIndex
}
