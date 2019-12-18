pub struct Person {
    pub var fullName: String

    init(firstName: String, lastName: String) {
        self.fullName = firstName.concat(" ").concat(lastName)
    }
}
