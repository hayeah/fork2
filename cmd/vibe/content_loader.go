package main

// ContentLoader loads text from configured sources.
type ContentLoader interface {
	Content() (string, error)
}
