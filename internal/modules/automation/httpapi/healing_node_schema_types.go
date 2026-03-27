package httpapi

type healingNodeSchemaResponse struct {
	InitialContext map[string]healingSchemaObject   `json:"initial_context"`
	Nodes          map[string]healingNodeDefinition `json:"nodes"`
}

type healingSchemaObject struct {
	Type        string                           `json:"type,omitempty"`
	Description string                           `json:"description,omitempty"`
	Properties  map[string]healingSchemaProperty `json:"properties,omitempty"`
}

type healingSchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type healingNodeDefinition struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Config      map[string]healingConfigField `json:"config"`
	Ports       healingNodePorts              `json:"ports"`
	Inputs      []healingNodeIO               `json:"inputs"`
	Outputs     []healingNodeIO               `json:"outputs"`
}

type healingConfigField struct {
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

type healingNodePorts struct {
	In       int                     `json:"in"`
	Out      int                     `json:"out"`
	OutPorts []healingNodePortOption `json:"out_ports,omitempty"`
}

type healingNodePortOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Condition string `json:"condition,omitempty"`
}

type healingNodeIO struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
}
