package data

// Book data struct
type Book struct {
	Id     int
	Title  string
	Author string
}

// InsertBook inserts a book
func (d *DAL) InsertBook(b *Book) (*Book, error) {
	err := d.db.Insert(b)
	return b, err
}

// SelectBooks returns all books
func (d *DAL) SelectBooks() ([]Book, error) {
	var books []Book
	_, err := d.db.Query(&books, `
		SELECT *
		FROM books
	`)
	return books, err
}

// SelectBookByID returns a book by id
func (d *DAL) SelectBookByID(ID string) (*Book, error) {
	var book Book
	_, err := d.db.QueryOne(&book, `
		SELECT *
		FROM books
		WHERE id = ?0
	`, ID)
	return &book, err
}
