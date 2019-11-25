let speed = "maximum"

struct Car {
    pub var model: String
    pub var make: String
    pub var trim: String

    init(fullname: String) {
        self.model = ""
        self.make = ""
        self.trim = ""
    }

/*
    init(params: [String;3]) {
        self.model = params[0]
        self.make = params[1]
        self.trim = params[2]
    }

    init(model:String, make:String, trim:String) {
        self.model = model
        self.make = make
        self.trim = trim
    }
    */
}

struct Fleet {
    pub let cars: [Car]

    init(car1: Car, car2: Car?, car3: Car?) {
        self.cars = [car1]
    }
}

event FenderBender(where place: String, cost: Int)
