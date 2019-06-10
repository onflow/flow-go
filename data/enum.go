package data

type Status int

const (
	PENDING Status = iota
	FINALIZED
    REVERTED
	SEALED
)

func (s Status) String() string {
    return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

