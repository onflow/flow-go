package automate

type Values struct {
	Branch       string       `yaml:"branch"`
	Commit       string       `yaml:"commit"`
	Defaults     Defaults     `yaml:"defaults"`
	Access       Access       `yaml:"access"`
	Collection   Collection   `yaml:"collection"`
	Consensus    Consensus    `yaml:"consensus"`
	Execution    Execution    `yaml:"execution"`
	Verification Verification `yaml:"verification"`
}

type ContainerPorts struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
}
type Requests struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}
type Limits struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}
type Resources struct {
	Requests Requests `yaml:"requests"`
	Limits   Limits   `yaml:"limits"`
}
type ServicePorts struct {
	Name       string `yaml:"name"`
	Protocol   string `yaml:"protocol"`
	Port       int    `yaml:"port"`
	TargetPort string `yaml:"targetPort"`
}
type Defaults struct {
	ImagePullPolicy string           `yaml:"imagePullPolicy"`
	ContainerPorts  []ContainerPorts `yaml:"containerPorts"`
	Env             []interface{}    `yaml:"env"`
	Resources       Resources        `yaml:"resources"`
	ServicePorts    []ServicePorts   `yaml:"servicePorts"`
	Storage         string           `yaml:"storage"`
}
type Env struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
type NodeDetails struct {
	Args   []string `yaml:"args"`
	Env    []Env    `yaml:"env"`
	Image  string   `yaml:"image"`
	NodeID string   `yaml:"nodeId"`
}

type Access struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}

type Collection struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}

type Consensus struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}

type Execution struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}

type Verification struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}
