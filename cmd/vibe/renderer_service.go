package main

import "github.com/hayeah/fork2/render"

// RendererService exposes rendering operations used by the pipeline.
type RendererService interface {
    LoadTemplate(path string) (*render.Template, error)
    RenderTemplate(t *render.Template, data render.Content) (string, error)
}
