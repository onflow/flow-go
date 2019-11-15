struct Car {
    pub var model: String
    pub var make: String

    pub var trim

    init(firstName: String, lastName: String) {
        self.fullName = firstName + " " + lastName
    }
}
