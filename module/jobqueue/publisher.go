package jobqueue

import (
	"github.com/dgraph-io/badger/v2"
)

type Publisher struct {
	storage *badger.DB
	jobName []byte
}

// func (p *Publisher) Publish(job Job) error {
// 	jid := job.ID()
// 	tx := p.storage.NewTransaction(true)
// 	defer tx.Discard()
//
// 	// ensure the job is unique
// 	_, err := tx.Get([]byte(jid))
// 	if err == nil {
// 		return fmt.Errorf("job already exist: %w", storage.ErrAlreadyExists)
// 	}
//
// 	err = tx.Set([]byte(jid), []byte{})
// 	if err != nil {
// 		return fmt.Errorf("could not set: %w", err)
// 	}
//
// 	// increment the index
// 	index, err := tx.Get(p.jobName)
// 	if err != nil {
// 		return fmt.Errorf("could not get current index: %w", err)
// 	}
//
// 	// TOFIX
// 	// next = index + 1
// 	var next []byte
//
// 	err = tx.Set(p.jobName, next)
// 	if err != nil {
// 		return fmt.Errorf("could not update next index: %w", err)
// 	}
//
// 	// put the element at that index
// 	key := nextJobKey(p.jobName, next)
// 	err = tx.Set(key, jid)
// 	if err != nil {
// 		return fmt.Errorf("could not store job: %w", err)
// 	}
//
// 	err = tx.Commit()
// 	if err != nil {
// 		return fmt.Errorf("failed to commit tx: %w", err)
// 	}
//
// 	return nil
// }
