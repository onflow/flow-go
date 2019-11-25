/*

// TODO Currently events cannot contain resources as field, but this is being worked on

struct Seed {
    pub let id: Int

    init(id: Int) {
        self.id = id
    }
}


resource Tomato {
    pub let seeds: [Seed]

    init(seeds: [Seed]) {
        self.seeds = seeds
    }
}

event Chopped(tomato: Tomato, seeds_left: [Seed])

event Throw(where tomato: <-Tomato, how_far distance: UInt16)

event Bought(tomatoes: {String: [Tomato?]})
*/

event Sow(seed: String, times: Int)
