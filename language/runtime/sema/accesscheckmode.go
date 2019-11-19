package sema

//go:generate stringer -type=AccessCheckMode

type AccessCheckMode int

const (
	AccessCheckModeStrict AccessCheckMode = iota
	AccessCheckModeNotSpecifiedRestricted
	AccessCheckModeNotSpecifiedUnrestricted
	AccessCheckModeNone
)
