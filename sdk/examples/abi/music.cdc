
pub struct Artist {
    pub let name: String
    pub let members: [String]?
    pub let country: String

    init(name:String, members: [String]?, country: String) {
        self.name = name
        self.members = members
        self.country = country
    }
}

pub resource Album {
    pub let artist: Artist
    pub let name: String
    pub let year: UInt16
    pub let rating: UInt8?

    init(artist:Artist, name:String, year:UInt16, rating:UInt8?) {
        self.artist = artist
        self.name = name
        self.year = year
        self.rating = rating
    }
}

/*
pub resource AlbumHolder {
    pub let albums: <-[Album]

    init(albums: <-[Album]) {
        self.albums <- albums
    }

    destroy {
        destroy self.albums
    }
}
*/

pub fun getAlbums(): @[Album] {
    //UNICODE
    let kraftwerk = Artist(name: "Kraftwerk", members: ["Ralf Hütter","Fritz Hilpert","Henning Schmitz","Falk Grieffenhagen"], country: "Germany")
    let giorgio = Artist(name: "Giorgio Moroder", members: nil, country: "Italy")

    let autobahn <- create Album(artist: kraftwerk, name: "Autobahn", year: 1974, rating: nil)
    let from_here <- create Album(artist: giorgio, name: "From Here to Eternity", year: 1977, rating: 5)
    let man_machine <- create Album(artist: kraftwerk, name: "Die Mensch-Maschine", year: 1978, rating: 4)

    return <-[<-autobahn, <-man_machine, <-from_here]
}

pub fun main():@[Album] {
    return <-getAlbums()
}
