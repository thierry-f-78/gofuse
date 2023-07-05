package main

type State struct {
	Version int `json:"version"`
	Repo string `json:"repo"`
	Refspec string `json:"refspec"`
	Id string `json:"id"`
	Files []string `json:"files"`
}
