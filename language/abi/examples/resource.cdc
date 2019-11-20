struct Colour {
    let r: UInt8
    let g: UInt8
    let b: UInt8

    init(r: UInt8, g: UInt8, b: UInt8) {
        self.r = r
        self.g = g
        self.b = b
    }
}

resource Banana {
    let colour: Colour

    init(colour: Colour) {
        self.colour = colour
    }
}

resource BunchOfBananas {
    let bananas: [Banana]

    init(bananas: [Banana]) {
        self.bananas = bananas
    }
}
