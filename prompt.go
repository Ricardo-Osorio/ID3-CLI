package main

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

type MusicMetadata struct {
	Artist   string
	SongName string
	Album    string
	Score    float64
	Sources  int
}

func (m *MusicMetadata) Copy() MusicMetadata {
	return MusicMetadata{
		Artist:   m.Artist,
		SongName: m.SongName,
		Album:    m.Album,
		Score:    m.Score,
		Sources:  m.Sources,
	}
}

func PromptSelectMatch(filename string, input []MusicMetadata) (int, error) {
	templates := &promptui.SelectTemplates{
		Label:    "Select match for: {{ .FileName | red }}",
		Active:   "▸ {{ .Artist | green }} - {{ .SongName | green }}",
		Inactive: "  {{ .Artist | cyan }} - {{ .SongName | cyan }}",
		Details: `
--------- Details ----------
{{ "Artist:" | faint }}	{{ .Artist }}
{{ "Title:" | faint }}	{{ .SongName }}
{{ "Album:" | faint }}	{{ .Album }}
{{ "Score:" | faint }}	{{ .Score }}
{{ "Sources:" | faint }}	{{ .Sources }}`,
	}

	prompt := promptui.Select{
		Label:        struct{ FileName string }{FileName: filename},
		Items:        input,
		Templates:    templates,
		Size:         6,
		HideSelected: true,
	}

	index, _, err := prompt.Run()
	if err != nil {
		fmt.Printf("failed to run prompt %s\n", err)
		return -1, err
	}

	return index, nil
}

type MusicTags struct {
	Tag   string
	Value string
}

func PromptSelectTag(filename string, input []MusicTags) (int, error) {
	var index int

	templates := &promptui.SelectTemplates{
		Label: "Edit label for: {{ .FileName | red }}",
		Active: "{{`▸`|green}} {{ .Tag | red }}{{if .Value}}:{{end}}	{{ .Value | red }}",
		Inactive: "  {{ .Tag | cyan }}{{if .Value}}:{{end}}	{{ .Value | cyan }}",
	}

	prompt := promptui.Select{
		Label:        struct{ FileName string }{FileName: filename},
		Items:        input,
		Templates:    templates,
		Size:         4, // match number of tags + 1 (option to continue)
		HideSelected: true,
	}

	index, _, err := prompt.Run()
	if err != nil {
		fmt.Printf("failed to run prompt %s\n", err)
		return -1, err
	}

	return index, nil
}

func PromptNewValue(oldValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:       "New value",
		Default:     oldValue,
		HideEntered: true,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return "", err
	}
	return result, nil
}
