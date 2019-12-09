pub struct Colour {
    pub let r: UInt8
    pub let g: UInt8
    pub let b: UInt8

    init(r: UInt8, g: UInt8, b: UInt8) {
        self.r = r
        self.g = g
        self.b = b
    }
}

pub resource Banana {
    pub let colour: Colour

    init(colour: Colour) {
        self.colour = colour
    }
}

pub resource BunchOfBananas {
    pub let bananas: <-[Banana?]

    init(bananas: <-[Banana]) {
        self.bananas <- bananas
    }

    destroy() {
        destroy self.bananas
    }
}
