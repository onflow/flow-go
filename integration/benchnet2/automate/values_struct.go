package automate

type Values struct {
	Branch       string      `yaml:"branch"`
	Commit       string      `yaml:"commit"`
	Defaults     EmptyStruct `yaml:"defaults"`
	Access       NodesDefs   `yaml:"access"`
	Collection   NodesDefs   `yaml:"collection"`
	Consensus    NodesDefs   `yaml:"consensus"`
	Execution    NodesDefs   `yaml:"execution"`
	Verification NodesDefs   `yaml:"verification"`
}

type EmptyStruct struct {
}
type ContainerPorts struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
}
type Resource struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}
type Resources struct {
	Requests Resource `yaml:"requests"`
	Limits   Resource `yaml:"limits"`
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

type NodesDefs struct {
	Defaults Defaults               `yaml:"defaults"`
	Nodes    map[string]NodeDetails `yaml:"nodes"`
}
