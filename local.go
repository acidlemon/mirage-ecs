package main

import "time"

/*
type ECSInterface interface {
	Run()
	Launch(subdomain string, option map[string]string, taskdefs ...string) error
	Logs(subdomain string, since time.Time, tail int) ([]string, error)
	Terminate(subdomain string) error
	TerminateBySubdomain(subdomain string) error
	List(status string) ([]Information, error)
}
*/

type ECSLocal struct {
	// ECSLocal is a local mock ECS implementation for Mirage.
}

func NewECSLocal(cfg *Config) *ECSLocal {
	// NewECSLocal returns a new ECSLocal instance.
	return &ECSLocal{}
}

func (ecs *ECSLocal) Run() {
	// Run starts the ECSLocal instance.
}

func (ecs *ECSLocal) List(status string) ([]Information, error) {
	// List returns a list of ECSInfo.
	return []Information{}, nil
}

func (ecs *ECSLocal) Launch(subdomain string, option map[string]string, taskdefs ...string) error {
	// Launch launches a new ECS task.
	return nil
}

func (ecs *ECSLocal) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	// Logs returns logs of the specified subdomain.
	return []string{}, nil
}

func (ecs *ECSLocal) Terminate(subdomain string) error {
	// Terminate terminates the specified subdomain.
	return nil
}

func (ecs *ECSLocal) TerminateBySubdomain(subdomain string) error {
	// TerminateBySubdomain terminates the specified subdomain.
	return nil
}
