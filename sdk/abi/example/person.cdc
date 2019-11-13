struct Person {
    pub var fullName: String

    init(firstName: String, lastName: String) {
        self.fullName = firstName + " " + lastName
    }

    init(fullName: String) {
        self.fullName = fullName
    }
}
