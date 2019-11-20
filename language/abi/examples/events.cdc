struct Seed {
    pub id: Int

    init(id: Int) {
        self.id = id
    }
}

resource Tomato {
    pub seeds: [Seed]

    init(seeds: [Seed]) {
        self.seeds = seeds
    }
}

event Chopped(tomato: Tomato, seeds_left: [Seed])

event Throw(where tomato: Tomato, how_far distance: UInt16)

event Bought(tomatoes: {UInt: [Tomato?]})
