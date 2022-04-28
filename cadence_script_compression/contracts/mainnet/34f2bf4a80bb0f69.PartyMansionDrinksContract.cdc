/**
*  SPDX-License-Identifier: GPL-3.0-only
*/
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61
import MetadataViews from 0x1d7e57aa55817448
                                                                                            
//                               ..`                                                                  
//                              +..o                                                                  
//                              :::/                                      `                           
//                                `                         .os:.        `y+../y.                     
//                                         ++./.          `o++o/o`        +mmdms                      
//                                       `:yNmd.         -oy//+o-       -ohmmmmdo.                    
//                                ./`    `-:my+/       `o+:ss/+         ---/mm/:::                    
//                               :yd.       /         .sd-``::              so                        
//                               +::+`              `os/+:+.                                          
//                                `oh`             -+ho.+y:     -:::-                                 
//                             -sdmh-            `yd:+-+:      :/   -/                                
//                             o-++              `ys``+:       .+. `+-                                
//                              -+s                              -:-`                                 
//                           .+shd:   `:::                                                            
//                           s+h/`    +.`+.                                     `-.                   
//                           -/:o     `-:.                                     -dmm:                  
//                         `:+sd/               -.-h.                          `sys.                  
//                        `ody+-                -hmmy+.        ::::              `                    
//                        `++    .--            -ysdo`         o..o                       /:-o-       
//                              `dmmo           `  ..          `..`  `o`                  :s-++-      
//                     `..`      +ys-                 `.`            //+..::             ./:+o..      
//                    `y:.-:-`                       .y:/:``      /+/:``-++.                .-        
//                    omh`  .:/.` ```             -::`++.dho      `.-+ ..o.                           
//                   /mmmh.    -//--:/           -m::/.o+-.`        `s/--:s`                          
//                  -mmmms/:    +.   /-      -::  :s-ds:`           `:               `                
//                 `dmmms  -/`  .+-.:s.     +d-//`.y/                             :sso/:+o:           
//                 ymmms    `/:   `.``//-+:+`/s:dh/`                        /yss/+mm../+dy.           
//                ommmo       sh:      `+d-:yoy+       .-`           `+sso:sNy `/syo                  
//               /mmmo      :dmmmy-     .-s/+:`      -/.`-+`       ``yN/ ./hy-                        
//              -mmm+     .smmmmmmmy:  /mms//`       o`   +.      `+ohs.                              
//             `dmm+     +dmmmmmmmds-/:.``  `/-      `/:-:-          `                                
//             ymm+    -hmmmmmmmh/`   `::-    :/        `                                             
//            omm/   .smmmmmmds-`      ./hho:` -/                                                     
//           /mm/   /dmmmmmh+.     `:ohdmmmmmhs/:                                                     
//          -mm/  -ymmmmms:`   `-/ydmmmmmdyo:.`                                                       
//         `hm: `ommmmh+.   .:shmmmmdho/-`                                                            
//         ym: :hmmmy:` `-+ydmmdhs/-.                                                                 
//        od:.ymmdo-`./sdmdhs+:.                                                                      
//       :d:+mmy/-:ohddy+:.`                                                                          
//      .dohds++shho/-`                                                                               
//     `hdmhyso/-`                                                                                    
//     sds/-`                                                                                         
//     `                                                                                              

// PartyMansionDrinksContract
//
// "To Imgur! Full of truth and full of fiction, please raise your glasses to 
// our unbreakable addiction."
//                                                                    
pub contract PartyMansionDrinksContract : NonFungibleToken {
    
    // Data 
    //
    // The fridge contains the beers to be served
    access(self) var fridge: [DrinkStruct]

     // The shaker contains the LongDrinks to be served
    access(self) var shaker: [DrinkStruct]

    // The whiskyCellar contains the whisky to be served
    access(self) var whiskyCellar: [DrinkStruct] 

    // FIFO
    pub var firstIndex: UInt32;

    // totalSupply
    pub var totalSupply: UInt64

    // last call (stop minting)
    pub var lastCall: Bool

    // Beer price
    pub var beerPrice: UFix64

    // Long Drink Price
    pub var longDrinkPrice: UFix64

    // Whisky Price
    pub var whiskyPrice: UFix64
    
    // Private collection storage path
    pub let CollectionStoragePath: StoragePath
    // Public collection storage path
    pub let CollectionPublicPath: PublicPath
    // Admin storage path
    pub let BarkeeperStoragePath: StoragePath

    // Events
    //
    // Event to be emitted when the contract is initialized
    pub event ContractInitialized()
    // Event to be emitted whenever a Drink is withdrawn from a collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Event to be emitted whenever a Drink is deposited to a collection
    pub event Deposit(id: UInt64, to: Address?)
    // Event to be emitted whenever a new Drink is minted
    pub event Minted(id: UInt64, title: String, description: String, cid: String)
      // Event to be emitted whenever a new airdrop happens
    pub event Airdropped(id: UInt64)

    //                                               ```./+++/.                                           
    //                                        `-:::/+++oyy/..+++o+-                                       
    //                                       /s:--/.`   -+:-.   ./y+/-`                                   
    //                                    -//o.     .   `       ````./y:`                                 
    //                                `:/o/.`    .     . `   `-/++:`  .:+o-                               
    //                               /o:.`  `   :.  . `:`.:/yo/--+yo:.`  -h-                              
    //                              :y`  `. `.      `:+/. `/s`   ` .:+oo. :y                              
    //                              +o`  ` ``  `   `  `-++/:` `  `     :y/.h.                             
    //                             `so`  ` ``  .- .:   -d.    `  `  .` ..od+                              
    //                             o+ `   `   `:-./- -oy:` `` -``. `.    .o+                              
    //                             o/ -   `  .`-+s-   d- ` -  ` .` `. .`  :y                              
    //                             d/`    :`      ```/d  .   ::  . .`  `  .h                              
    //                             :y-    `   .  .dso/++++++//::-/` - .`  .h                              
    //                              `s: ``  o`  .hod..mmmmmmmmmmmmmdys+.  -y                              
    //                               `y   -`  :+d:ym.:mmmmhmmmm+smmmmmm/s/+o                              
    //                                -h```   :s- mm-/mmmmdmmmmmmmmmmmm-m-y:                              
    //                                 +s:-- `h`  dm//mmmmmmymmmmmdhmmd+d.d                               
    //                                  oy+  `h   yms-mmmmmmmmmmmmmmmmsd++o                               
    //                                  `d.   y-  +md.mmdommmmddmmmmmmym.d.                               
    //                                   s+-  +o  .mm-mmmmmmmmmmmmmmmmmo/s                                
    //                                   .d//.y/   dm+ymmmmdmmmhmmmmmmm.h-                                
    //                                    s+:/:    omhommmmdmmmhmmmmmms/y                                 
    //                                    .h.+     -mm+mmmmmmmmmmyhmmm.h-                                 
    //                                     y:/      dmommmmsmmmmmdmmmy:y                                  
    //                                     :y.      omhdmmmmmmmmmdmmm:y:                                  
    //                                      d.      -mmdmmmmmymmmmmmd.d`                                  
    //                                      o+`      mmmmmmmmmmmmmmmo/s                                   
    //                                      :y-`     ymmmmddmmmmmmmm:y:                                   
    //                                      `m`/     +mmmmmmmmmmmmmm`d`                                   
    //                                       d.o     -mmmmmmmmdmmmmd`m                                    
    //                                       h-y     `mmmmmhmmmmmmmh-h                                    
    //                                       y:y`     mmmmmmmmdmmmdy:y                                    
    //                                       y:y.     dmmmmmmmmmmsmy:y                                    
    //                                       y:y.     hmmmmmmymmm/mh-h                                    
    //                                       d.h`     dmmmmmmmmmm-md.d                                    
    //                                      `m`m`    `mmmmmmmmmmm-hm`m`                                   
    //                                      -h-m`    -mmmmmmmmmmm:om-y:                                   
    //                                      +o+m. `-:oyyhhhdmmmmm+-m++o                                   
    //                                      h-hm+ `-:/+osydhhmmmm+-mh.d                                   
    //                                     .d.mmm:`  :osyhhmmmmmmydmm.h-                                  
    //                                     o+ommhhyo+hmmmmmmmmmddhhmmo/s                                  
    //                                     m`dmmmmhhysssooosssyhhmmmmm`m`                                 
    //                                     y/odddmdhdddmmmmmdddhdmdddo/y                                  
    //                                     `+o/++///.`.-----.`.://///o+`                                  
    //                                       `:+o+:.```-/++:.``.:+o+/`                                    
    //                                           .-/+++++++++++/-.                                        

    // Drink data structure
    // This structure is used to store the payload of the Drink NFT
    //
    // "Here’s to a night on the town, new faces all around, taking the time
    // to finally unwind, tonight it’s about to go down!"
    //
    pub struct DrinkStruct {

        // ID of drink
        pub let drinkID: UInt64
        // ID in collection
        pub let collectionID: UInt64
        // Title of drink
        pub let title: String
        // Long description
        pub let description: String
        // CID of the IPFS address of the picture related to the Drink NFT
        pub let cid: String
        // Drink type
        pub let drinkType: DrinkType
        // Drink rarity
        pub let rarity: UInt64
        // Metadata of the Drink
        pub let metadata: {String: AnyStruct}

        // init
        // Constructor method to initialize a Drink
        //
        // "To nights we will never remember, with the friends we will never forget."
        //
        init( 
            drinkID: UInt64,
            collectionID: UInt64,
            title: String,
            description: String,
            cid: String,
            drinkType: DrinkType,
            rarity: UInt64,
            metadata: {String: AnyStruct}
            ) {
            self.drinkID = drinkID
            self.collectionID = collectionID
            self.title = title
            self.description = description
            self.cid = cid
            self.drinkType = drinkType 
            self.rarity = rarity
            self.metadata = metadata
        }
    }

    // Drink Enum
    //
    // German
    // „Ach, mir tut das Herz so weh,
    // wenn ich vom Glas den Boden seh.“
    //
    pub enum DrinkType: UInt8 {
        pub case Beer
        pub case Whisky
        pub case LongDrink
    }

    // Drink NFT
    //
    // "Here’s to those who wish us well. And those that don’t can go to hell."
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        // NFT id
        pub let id: UInt64
        // Data structure containing all relevant describing data
        pub let data: DrinkStruct
        // Original owner of NFT
        pub let originalOwner: Address

        // init 
        // Constructor to initialize the Drink
        //
        // "May we be in heaven half an hour before the Devil knows we’re dead."
        //
        init(drink: DrinkStruct, originalOwner: Address) {
            PartyMansionDrinksContract.totalSupply = PartyMansionDrinksContract.totalSupply + 1
            let nftID = PartyMansionDrinksContract.totalSupply

            self.data = DrinkStruct(
            drinkID: nftID,
            collectionID: drink.collectionID,
            title: drink.title,
            description: drink.description,
            cid: drink.cid,
            drinkType: drink.drinkType,
            rarity: drink.rarity,
            metadata: drink.metadata)

            self.id = UInt64(self.data.drinkID)
            self.originalOwner = originalOwner
        }

        // name
        //
        // "If the ocean was beer and I was a duck, 
        //  I’d swim to the bottom and drink my way up. 
        //  But the ocean’s not beer, and I’m not a duck. 
        //  So raise up your glasses and shut the fuck up."
        //
        pub fun name(): String {
            return self.data.title
        }

        // description
        //
        // "Ashes to ashes, dust to dust, if it weren’t for our ass, our belly would bust!"
        //
        pub fun description(): String {
            
            return "A "
                .concat(PartyMansionDrinksContract.rarityToString(rarity: self.data.rarity))
                .concat(" ")
                .concat(self.data.title)
                .concat(" with serial number ")
                .concat(self.id.toString())
        }

        // imageCID
        //
        // "Here’s to being single, drinking doubles, and seeing triple."
        //
        pub fun imageCID(): String {
            return self.data.cid
        }

        // getViews
        //
        // "Up a long ladder, down a stiff rope, here’s to King Billy, to hell with the pope!"
        //
        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        // resolveView
        //
        // "Let us drink to bread, for without bread, there would be no toast."
        //
        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name(),
                        description: self.description(),
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.imageCID(),
                            path: nil
                        )
                    )
            }
            return nil
        }
    }

    // Interface to publicly acccess DrinkCollection
    //
    // "Out with the old. In with the new. Cheers to the future. And All that we do"
    //
    pub resource interface DrinkCollectionPublic {

        // Deposit NFT
        // Param token refers to a NonFungibleToken.NFT to be deposited within this collection
        //
        // Belgian
        // "Proost op onze lever"
        //
        pub fun deposit(token: @NonFungibleToken.NFT)

        // Get all IDs related to the current Drink Collection
        // returns an Array of IDs
        //
        // Czech
        // "Na EX!"
        //
        pub fun getIDs(): [UInt64]

        // Borrow NonFungibleToken NFT with ID from current Drink Collection 
        // Param id refers to a Drink ID
        // Returns a reference to the NonFungibleToken.NFT
        //
        // Czech
        // "Do dna!"
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        // Eventually borrow Drink NFT with ID from current Drink Collection
        // Returns an option reference to PartyMansionDrinksContract.NFT
        //
        // Czech
        // "Na zdraví!"
        //
        pub fun borrowDrink(id: UInt64): &PartyMansionDrinksContract.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Drink reference: The ID of the returned reference is incorrect"
            }
        }
    }


    //ddxxkO000OOO0000OOOOO0000000O00KXXNWWNX0Oxoc:::;;;;;::::c:ccclooooooooooooooooooooddddddx0NWWWWNXK00
    //xxxkkkkkkkkkxxddddddddxxxxxxxxxxxdoolc;,,;:clloddoodxxddxdollllccloooooooooodxxkOKXXKKXXXNWMMMWXK0OO
    //ooooooooooooooooooooooooooooooc:;,..',:codkOOOO00OOO00OO0000000OxddolldddddOXNNWWWWWWNNNWWWWWWNK0OOO
    //oooooooooooooooooooooooooool:,'...';:lodddxxkkkO00OOO000OO00000K0OKKkdx0KKXWWWWWNNXXK00KXXNNNXK00OOO
    //oooooooooooooooooooooooool;'....',;:::cloddxkOOO0KKK000K0O0000000000K0OOKWWWWWWNNNXK0OOOO000000OOOOk
    //oooooooooodddooooooooool:'....',,,,,,:clddkkOO000000K0KKKKKKKKKKKK00000kkKNNWWWWWWNXK0Okkkkkkkxxxxxx
    //ooooooooooooooooooooooc,......'''',,;:cloodxxxxkkkOOOOOOOOO00O000KKK0000xkKXXNNWNNXK00Okkkkkkkxxxxxx
    //ooooooooodooddoooddooc.........'',;;:::ccclllllllclccllcccclooxkO000KKK0kxkOkO00OOOOOOOO000OOkkxxxxx
    //ddddddddddxkkxddddddl'....'..',;;;;:::;,,,,,,,,,,,,,,,'''...'',:loxO000KkxkOOOOOOO000000000Okxdddddd
    //kkkkkkOO0KXXXK0kxxdo;...''.',,;;;;,,,'...............         ...,:lxkk0kdkkkOOOOOOOOOOOOOkkkxxxxddd
    //KKKKKKKXNNNNNNNXKKOd,..',',,;;;,,'........                       ...;lxdddxdxxxxxxxxxxxxxxxxxkxxddxx
    //kO0KKXXXNNWWWWWWWWWKc..,;;::;,'........  .....      ....'',,;;,.. ...;codddddddddddddddddddddddddddd
    //odk0KXXXXXNWWMMMMMWM0;,::::;,........  .','..   .. ..';;:cloddo;.....:oddddddddddddddddddddddddddddd
    //odO0K0000KXNWWWWMMMWWKdlll:'..... .....''.. ...''.....';cloddxxo;...,ldddddddddddddddddddddddddddddd
    //oxkOkxxkO0KKKKXXNWWWWWNX0d,...   ... ....  ....'''.'...,:coddxkkl'..;odddddxxxddddddddddddxddddddddd
    //ddxddodxkOOOOkkkO00KKXXXNO, ..   .......   ..........'',;:codxxkx:..cddddddddddddddddddddxxxdddddddd
    //kkkxxdxxxxkkxxxxkkkkOOOO0k' ..   .. ..     .......''''',;::loxxxkd;,odddddddddxxkOOkxddddddddddddddd
    //kkkkkkkkkkkkkkkkkkOOOO00KO, ..   ..       ...........'',,;:cldxxxxdlodddddxxxk0KXXXXK0OOO00OOkkkxxxx
    //xxxxxxxxxxxxxxxxxxxxxk00Oo,...   .        ............'',;:clodxxxkkooddxxkOO0KKK0OO0000XXK0000OOkkk
    //ddddddddddddddddddddddxxl:c,..   .  .   .. .............',;:cloddxxxxooxxxxxxxkkkkkkkkkkO0OkkOOkxxxx
    //dxxxdddddddddddxddxxxddxdl;...   .  . .........'..........',;:ldxxxxkkooxxxdddddddxxxdxxxxxxxxxxxxxx
    //xxxxkkkkOOOOkkkkkkxxkxxo::cl:. ....  ...........'''........'',:cccloddlcdxxdddddxxxxxxxdxxxxxxxxxxxx
    //kkkkkOO00KKK00OOOOOOOkxllxdxc. .............................',;,'';::;;cdxxddxxdxxxxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxxkxxxxxxxxxxl. ...  ................'......'',;:cccllllcdxxdddxxxxxxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxxxxxxxxxxxxxl...'.  ...........''......''',,,;::clllolloddddddddddxxxdxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxxxxxxxxxxxxxo'.''... ........'',,;;;'..',;;;::::clllcccoollllllllldxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxxxxxxxxxxxxxo'.,'...........'',;;;;:;,,''',;;::cclllllodxddddddddddxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxxxxxxxxxxxdoc...............',,;;:::cc:,..'',;;;:cccldxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxkxxxxxxdolc:,...............',,;;:::cclc;''...''';;:ldxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    //xxxxxxxxxxxxxxxxkkxxkkdc::;:,...............',,;;::::clooo:,'.....,;ldxxxxxxxxxxxxxxxxxxxxxxxxxxxkkk
    //kkkkkkkkkkkkkkkkkkkxxko::;,,'................',,,;;:::cloddl:,.',;:coxkxxxxxxxxxxxxxxxxkxxxxkkkkkkkk
    //kkkkkkkkkkkkkkkkkkkkkkdl:;,''''.'...........''',,,,;::clooooc,.coodkkkkkxxxxxxxxkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkkkkkkkkxl:;;;;,,'''...'...''',,,,,,;;;;cllolc;';oxkkxdxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkkkkkkxdlc::::;;:::;;,'''''''...,;:::::;;:cc:,';ldOXX0Oxxdxkkkkkkkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkkkkxdlc:::::::::c::cc:;,',,,'....'::col:;;,'';:cxKXXKKKKkccdkkkkkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkkkdc,,'''',,;;:clccllllc:;;;;;,,....,cooc'..'::lOXXXXKKK0l,coxkkkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkkd:;,,,,,'''''',;clllllolc:;::cc:....;clo:'',clo0XXXXKKKKd;cxxxkkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkkdllc:::ccccc:;,''',:lllooolc:clll;....:looc',lco0XXXXKKKKk;:OKxxkkkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkkkd:;,''...'',:lool;'',:loooodoclllll:'..'lddo;,lldKXXXXKKKKx;:OXkdkOkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkOkl,,.....'''',,;cddc,'':loodddoooollc:,..:dddc,ldkKXXNNK0K0o,c00xodOkkkkkkkkkkkkkkkkkkk
    //kkkkkkkkkkkkOx:........''',,;::lo:'.':oddddkkdollll:..;odxl;lokKXXXXK0KOc'ckxdookOkkOkkkkkkkkkkkkkkk
    //kkkkkkkkOkkkOx;.............';::c;'..,coooxOOxlllllc,..,:llclokXXXNXK0Kkc';lodllxOkkOOkkOkkkkkkkOOOO
    //OOOOOOkkOkkOOd'...     ......',,;;'..';lookOOxolllll,...',cllokXXNNXK00x:',:lc;;dOkkOOkOOOOOOOOOOOOO
    //OOOOOOOkOOOkOo;,...........''''.......';lxkOOkocllll:..'',:lloOXNNXKK00d;'.',,,;oOOkOOOOOOOOOOOOOOOO
    //OOOOOOOOkkOOko::,''''',;coolcc:;,'..'..,lkOOkxocclloc,.''';lloOXNNXKK0kd;'.,:ldddkOOOOOOOOOOOOOOOOOO
    //OOOOOOOOOOOOkl:;''''';codddoddxxdc..'..'okkkxxdlcllol;'''';lodkXNXXX0kkx:.,oxOOxdkOOOOOOOOOOOOOOOOOO
    //OOOOOOOOOOOOklc:,,,;:cooooddxkkkxc.....'lxxxxddlclloo;...';lddOXNXNKkkOx;.,okOOOxkOOOOOOOOOOOOOOOOOO
    //OOOOOOOOOOOOxllc::;;:cldxkO00KK0k:....',lddoooolclooo:..'':odd0XNXKOxk0d,'cxO00OdxOOOOOOOOOOOOOOOOOO
    //OOOOOOOOOOOOkolc;;;;;clodxk0KKKOkc.....';lllloollloll;..',:odx0XXKOxxOOl'',cloolcxOkOOOOOOOOOOOOOOOO
    //OOOOOOOOOOOOklc:;;,,;:ccloxO00Oxd:......,:cccoollllll;'',,:odx0X0Oxxk0k:...';::::dOOOOOOOOOOOOOOOOOO

    // Resource to define methods to be used by the barkeeper in order 
    // to administer and populate the fridge
    //
    // "Here’s to those who seen us at our best and seen us 
    // at our worst and cannot tell the difference."
    //
    pub resource Barkeeper {

        // Adds drinks to the fridge
        //
        // Polish
        // "Sto lat!"
        //
        pub fun fillFridge(
            collectionID: UInt64,
            title: String,
            description: String,
            cid: String,
            rarity: UInt64,
            metadata: {String: AnyStruct}
        ) {
            pre {
                cid.length > 0 : "Could not create Drink: cid is required."
            }

            PartyMansionDrinksContract.fridge.append(
                DrinkStruct( drinkID: 0 as UInt64,
                    collectionID: collectionID,
                    title: title,
                    description: description,
                    cid: cid,
                    drinkType: DrinkType.Beer,
                    rarity: rarity,
                    metadata: metadata))
        }

        // Replaces old drinks in fridge
        // because nobody serves old drinks to the party people
        //
        // Polish
        // "Za nas!"
        //
        pub fun replaceDrinkInFridge(
            collectionID: UInt64,
            title: String,
            description: String,
            cid: String,
            rarity: UInt64,
            metadata: {String: AnyStruct},
            index: Int
        ) {
        pre {
            cid.length > 0 : "Could not create Drink: cid is required."
            index >= 0 : "Index is out of bounds"
            PartyMansionDrinksContract.fridge.length > index : "Index is out of bounds."
        }

        PartyMansionDrinksContract.fridge[index] = DrinkStruct( 
            drinkID: 0 as UInt64,
            collectionID: collectionID,
            title: title,
            description: description,
            cid: cid,
            drinkType: DrinkType.Beer,
            rarity: rarity,
            metadata: metadata)
        }

        // removeDrinkFromFridge
        // because sometimes party people do change their taste
        //
        // Polish
        // “Za tych co nie mogą”
        //
        pub fun removeDrinkFromFridge(index: UInt64) {
            pre {
                PartyMansionDrinksContract.fridge[index] != nil : "Could not take drink out of the fridge: drink does not exist."
            }
            PartyMansionDrinksContract.fridge.remove(at: index)
        }

        // throwHouseRound
        // The barkeeper throws a house round and airdrops the Drink to a Recipient
        //
        // "Here’s to the glass we love so to sip,
        // It dries many a pensive tear;
        // ’Tis not so sweet as a woman’s lip
        // But a damned sight more sincere."
        //
        pub fun throwHouseRound(
            collectionID: UInt64,
            title: String,
            description: String,
            cid: String,
            rarity: UInt64,
            metadata: {String: AnyStruct},
            drinkType: DrinkType,
            recipient: &{NonFungibleToken.CollectionPublic},
            originalOwner: Address) {
            pre {
                cid.length > 0 : "Could not create Drink: cid is required."
            }
            // GooberStruct initializing
            let drink : DrinkStruct = DrinkStruct(
                drinkID: 0 as UInt64,
                collectionID: collectionID,
                title: title,
                description: description,
                cid: cid,
                drinkType: drinkType,
                rarity: rarity,
                metadata: metadata)
            recipient.deposit(token: <-create PartyMansionDrinksContract.NFT(drink: drink, originalOwner: originalOwner))
            
            emit Minted(id: PartyMansionDrinksContract.totalSupply, title: drink!.title, description: drink!.description, cid: drink!.cid)
            emit Airdropped(id: PartyMansionDrinksContract.totalSupply)
        }   

        // announceLastCall
        // Barkeeper shouts "Last Call"
        //
        // "May you always lie, cheat, and steal. Lie beside the one you love,
        // cheat the devil, and steal away from bad company."
        //
        pub fun announceLastCall(lastCall: Bool) {
            PartyMansionDrinksContract.lastCall = lastCall
        }

        // setDrinkPrices
        //
        // Mexico
        // "Por lo que ayer dolió y hoy ya no importa, salud!"
        //
        pub fun setDrinkPrices(beerPrice: UFix64, whiskyPrice: UFix64, longDrinkPrice: UFix64) {
            PartyMansionDrinksContract.beerPrice = beerPrice
            PartyMansionDrinksContract.whiskyPrice = whiskyPrice
            PartyMansionDrinksContract.longDrinkPrice = longDrinkPrice
        }
    }


    //                                  `--:/+oosyyyhhhhhhhhhhyysso+//:-.`                                 
    //                        `.:+sydmNMMNNmmdhhyyysssssssssyyyhhddmNNMMNmdhs+/-`                         
    //                   `-/sdNNNmdys+/:-````` ``              ```````.-/+syhmNNNmyo:`                    
    //                ./ymNNdy+:.``                                         ``.-+shmNNh+-                 
    //              .smNds:.`                                                     `.:odNNh:`              
    //             /mMh:`                                                             `-sNNy`             
    //            .NMs`                                                                  :NMs             
    //            :MM/                                                                   `mMd             
    //            -MMm/`                                                                .yMMy             
    //            `MMMMdo.`                                                         `.:yNMMMs             
    //             NMh+dNNds/-``                                               ```:ohmMNy/MM+             
    //             mMd  .+ymNMmdyo/-```                                  ``.:+sydNMNds:` .MM/             
    //             hMm      .:+ydmMMMNmhyso++/:--...````````...-::/+osyhdmNMMNmho/-`     :MM-             
    //             yMM            `.:/osyhdmmNMMMMMMMMMMMMMMMMMMMNNmdhyso+/-`            +MM.             
    //             oMM`                    `````...----::----...`````                    oMN              
    //             +MM-                                                                  yMm              
    //             :MM/                                                                  hMh              
    //             -MM+                                           .-:+-                  mMy              
    //             `MMs                                    `.-:+ssso//ys-                NMo              
    //              NMy          ``..--:/+os/`       `.-/+ssso/-.`    `/hs.             `MM+              
    //              mMd     /oossyyyssoo+::/dy-...:ossso/-.`            `+hs.           -MM:              
    //              hMm    `Nd:-.``         .ymmmNMMs`                    .odo.         :MM-              
    //              yMM    `mhy`              :mMMMyhs`                     -ymo`       +MM`              
    //              oMM.   `m`yh`              `sMMo`sd-                 `-oyydMdo-     oMN               
    //              /MM-   `m``sh`      ```.-/+osdMo  /m/             ./yhs/. sMMMMy.   yMm               
    //              :MM/   oMNhohd:+osyyyyso+/-.` do   -ms`       `:shy+.     hMMMMMh   hMh               
    //              .MM+   -NMMMMMMs-`            do    `yd.   -ohho-      .+mMMMMMM+   mMy               
    //              `MMs    .smMMMMMNho/-`        ds      +m+yhs:``    .:odNMMMMMNy-    NMo               
    //               NMy   o-``:shmMMMMMNNmhso+//:mNmho+/:-ym/::://oshmNNMMMMNdy/`-+/  `MM/               
    //               mMd   dNms/.``-/oyhdmNMMMMMMNMMMMMMMNNNMNNNNMMMMNmmhyo/-.-:odNM:  -MM:               
    //               hMm   yMM/+yhyo/:.....-://+ossyyyyhyyyyyssoo+/:---://+osymNMMMM.  :MM.               
    //               yMN`  sMMd:`.Nddddmdhyso++//::::::::::://++ooosssoooosydmNMMMMN`  +MM`               
    //               oMM.  oMMMNs.m+`.--:/+oyhdNMMMMMMNyo++hm+//:---:/ohmNMMMMMMMMMm   oMN                
    //               /MM-  /MMMMMmN+ `.-:+yhmNMMMMMMMMMNd+.oh `.:ohmNMMMMMMMMMMMMMMy   yMd                
    //               :MM/  `dMMMMMMhydmNMMMMMMMMMMMMMMMMMMNmmhdNMMMMMMMMMMMMMMMMMMM:   hMh                
    //               .MM+   .dMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMNo    mMs                
    //               `MMy`   `+mNMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMNh-    .NMo                
    //                NMMd/`   `-+ymNMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMNmy+.   `-sNMM/                
    //                mMMMMNy/`     .:+ydNMMMMMMMMMMMMMMMMMMMMMMMMMMMMmhs+:`    `:smMMmMM:                
    //                hMMMMMMMNho-`       `.:/oosyhhdddddddhhhyso+/-.       .:odMMMm+.+MM.                
    //                sMMMMMMMMMMMNds+:.`           ``  ``           `.:+shmNMMMMMMy  oMM                 
    //                oMMMMMMMMM-:oydNNNNdhso+/::---...-.--:::/+osyhmNNNNdyo/hMMMMMs  sMN                 
    //                /MMMMMMMMM.   `.-/osyhdmmNNNNNNNNNNNNNNNmmdhhyo+:..`   sMMMMMo  hMd                 
    //                :MMMMMMMMM-          ```...--:::::----....```          sMMMMM+  dMy                 
    //                .MMMMMMMMM:                                            sMMMMM/ `mMs                 
    //                 yNMMMMMMM:                                            sMMMMM//dMd-                 
    //                  /dMMMMMM/                                            sMMMMMmMmo.                  
    //                   `/hNMMMo`                                           yMMMMNmo.                    
    //                      -odNMmho:.``                                `.-/smMNms:`                      
    //                         ./sdNNNmhso/-.````  `       `  ````.-:+oydNNNmy+-                          
    //                             `-+shmNNMMNNmdhhyyyyssyyyhhddmNNMNNmhs+:.                              
    //                                    `-:/+ossyyhhhhhhhyysso+/:-.        



    // Collection
    // A collection of Drink NFTs owned by an account
    //
    // Polish
    // “Człowiek nie wielbłąd, pić musi”
    //
    pub resource Collection: DrinkCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        // Norwegian
        // "Vi skåler for våre venner og de som vi kjenner og 
        //  de som vi ikke kjenner de driter vi i. Hei, skål!"
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        // British
        // "Let's drink the liquid of amber so bright;
        //  Let's drink the liquid with foam snowy white; 
        //  Let's drink the liquid that brings all good cheer; 
        //  Oh, where is the drink like old-fashioned beer?"
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        // Irish
        // "May your blessings outnumber
        // The shamrocks that grow,
        // And may trouble avoid you
        // Wherever you go."
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @PartyMansionDrinksContract.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            // destroy old resource
            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        // Irish
        // "Here’s to a long life and a merry one.
        // A quick death and an easy one.
        // A pretty girl and an honest one.
        // A cold beer and another one."
        //
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        // French
        // “Je lève mon verre à la liberté.” 
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowDrink
        // Gets a reference to an NFT in the collection as a Drink,
        // exposing all data.
        // This is safe as there are no functions that can be called on the Drink.
        //
        // Spanish
        // "¡arriba, abajo, al centro y adentro!"
        //
        pub fun borrowDrink(id: UInt64): &PartyMansionDrinksContract.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &PartyMansionDrinksContract.NFT
            } else {
                return nil
            }
        }

        // destructor
        //
        // Russian
        // "Давайте всегда наслаждаться жизнью, как этим бокалом вина!"
        //
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        // Russian
        // "Выпьем за то, что мы здесь собрались, и чтобы чаще собирались!"
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    // Russian
    // "Выпьем за то, чтобы у нас всегда был повод для праздника!"
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }



    // +ooooooooooooooo+/.                                                           -/+syyyso/-`         
    // hNNNNNNNNNNNNNNNMMNd+`                                                    `/ymNMNmmddmNMNNh/`      
    // ````````````````.:yMMd.                                                 `omMNho:.`````.-+hNMmo`    
    //                    /NMm/                                               -dMNs-`            .sNMm:   
    //                     -dMNo`                                            :NMm:                 -dMN/  
    //                      `sMMh.                                          `mMN-                   .mMN. 
    //                        /NMm:                                         +MMo                     /MMs 
    //                         -dMNo`                                       :++.                     .MMd 
    //             /ssssssssssssyMMMhssssssssssssssssssssssssssssssssssssssssssssssssso`             -MMh 
    //              +NMMMMNNNNNNNNNNMMMNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNMMMMh.              sMM+ 
    //               .yMMN+````````-mMMs```````````````````````````````````````-dMMm/               /MMd` 
    //                 :mMMh.       `yMMh`                                    +NMMs`              `sMMd`  
    //                   oNMMo`       +NMm-                                 -dMMd-`+:           -sNMNo    
    //                    .hMMm:   +sssdMMNssssssssssssssssssssssssssssssssyMMN+  yMMNhs+///+ohmMMms.     
    //                      /mMMy. `sNMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMy.   `-ohmNMMMMMNmho:`       
    //                       `sMMN+` -dMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMd:         ``..--.``           
    //                         :dMMd- `+mMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMNo`                              
    //                          `+NMNs. .yMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMh-                                
    //                            .yMMm/  :dMMMMMMMMMMMMMMMMMMMMMMMMMMm/`                                 
    //                              /mMMy. `oNMMMMMMMMMMMMMMMMMMMMMMNs.                                   
    //                               `sNMN+` -hMMMMMMMMMMMMMMMMMMMMd:                                     
    //                                 -dMMd:  +mMMMMMMMMMMMMMMMMNo`                                      
    //                                   +NMMs` .yMMMMMMMMMMMMMMh.                                        
    //                                    .yMMm/  :dMMMMMMMMMMm/                                          
    //                                      :mMMh.  oNMMMMMMMs`                                           
    //                                        oNMMo` .hMMMMd-                                             
    //                                         .hMMm:`yMMN+                                               
    //                                           /mMMNMMy.                                                
    //                                            `yMMN:                                                  
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                             /MMd                                                   
    //                                ```````.....-oMMm--.....``````                                      
    //                              -/+oossyyyyyyyyyyyyyyyyyyyyssso+/:`                                  

    // Beware !!! These methods know only german drinking toasts !!!

    // checkIfBarIsSoldOut
    //
    // Before selling a drink the barkeeper needs to check the stock
    //
    // German
    // "Nimmst du täglich deinen Tropfen,
    //  wird dein Herz stets freudig klopfen,
    //  wirst im Alter wie der Wein,
    //  stets begehrt und heiter sein."
    //
    pub fun checkIfBarIsSoldOut(drinkType: DrinkType) : Bool {
        switch drinkType {
            case DrinkType.Beer:
                return (PartyMansionDrinksContract.fridge.length == 0)
            case DrinkType.Whisky:
                return (PartyMansionDrinksContract.whiskyCellar.length == 0)
            case DrinkType.LongDrink:
                return (PartyMansionDrinksContract.shaker.length == 0)
            default:
                panic("Undefined drink type.")
        }
        return false
    }   
    
    
    // getPriceByTypeFromPriceList
    //
    // German
    // "Der größte Feind des Menschen wohl,
    // das ist und bleibt der Alkohol.
    // Doch in der Bibel steht geschrieben:
    // „Du sollst auch deine Feinde lieben.“
    //
    pub fun getPriceByTypeFromPriceList(drinkType: DrinkType) : UFix64 {
        var price : UFix64 = 0.00
        switch drinkType {
            case DrinkType.Beer:
                price = self.beerPrice
            case DrinkType.Whisky:
                price = self.whiskyPrice
            case DrinkType.LongDrink:
                price = self.longDrinkPrice
            default:
                panic("Undefined drink type.")
        }
        return price
    }

    // getDrinkFromStorage
    //
    // German
    // "Moses klopfte an einen Stein,
    // da wurde Wasser gleich zu Wein,
    // doch viel bequemer hast du's hier,
    // brauchst nur rufen: Wirt, ein Bier!"
    //
    pub fun getDrinkFromStorage(drinkType: DrinkType) : DrinkStruct {
        var disposedDrink : DrinkStruct = DrinkStruct( 
            
            drinkID: 0 as UInt64,
            collectionID: 0 as UInt64,
            title: " ",
            description: " ",
            cid: " ",
            drinkType: drinkType,
            rarity: 0 as UInt64,
            metadata: {})

        switch drinkType {
            case DrinkType.Beer:
                disposedDrink =  PartyMansionDrinksContract.fridge.remove(at: PartyMansionDrinksContract.firstIndex)
            case DrinkType.Whisky:
                disposedDrink = PartyMansionDrinksContract.whiskyCellar.remove(at: PartyMansionDrinksContract.firstIndex)
            case DrinkType.LongDrink:
                disposedDrink = PartyMansionDrinksContract.shaker.remove(at: PartyMansionDrinksContract.firstIndex)
            default:
                panic("Undefined drink type.")
        }
        return disposedDrink
    }

    // buyDrink
    // Mints a new Drink NFT with a new ID and disposes it from the Bar
    // and deposits it in the recipients collection using their collection reference
    //
    // German
    // "Wer Liebe mag und Einigkeit, der trinkt auch mal ne Kleinigkeit."
    //
    pub fun buyDrink(recipient: &{NonFungibleToken.CollectionPublic}, address: Address, paymentVault: @FungibleToken.Vault, drinkType: DrinkType) {
        pre {
            PartyMansionDrinksContract.lastCall == false: "No minting possible. Barkeeper already announced last call."      
            PartyMansionDrinksContract.checkIfBarIsSoldOut(drinkType: drinkType) == false: "Bar is sold out"
            paymentVault.isInstance(Type<@FlowToken.Vault>()): "payment vault is not requested fungible token"
            paymentVault.balance >= PartyMansionDrinksContract.getPriceByTypeFromPriceList(drinkType: drinkType) : "Could not buy Drink: payment balance insufficient."
        }

        // pay the barkeeper
        let partyMansionDrinkContractAccount: PublicAccount = getAccount(PartyMansionDrinksContract.account.address)
        let partyMansionDrinkContractReceiver: Capability<&{FungibleToken.Receiver}> = partyMansionDrinkContractAccount.getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)!
        let borrowPartyMansionDrinkContractReceiver = partyMansionDrinkContractReceiver.borrow()!
        borrowPartyMansionDrinkContractReceiver.deposit(from: <- paymentVault.withdraw(amount: paymentVault.balance))

        // serve drink
        let disposedDrink : DrinkStruct = PartyMansionDrinksContract.getDrinkFromStorage(drinkType: drinkType)
        recipient.deposit(token: <-create PartyMansionDrinksContract.NFT(drink: disposedDrink!, address))
        emit Minted(id: PartyMansionDrinksContract.totalSupply, title: disposedDrink!.title, description: disposedDrink!.description, cid: disposedDrink!.cid)

        // close the wallet
        destroy paymentVault
    }

    //                                                 -:.                                                
    //                                                 :hd--                                              
    //                                                 -sMNm-                                             
    //                                                  `+MM+                                             
    //                                                   `NN.                                             
    //                                                   :Mm                                              
    //                                                  `mMh                                              
    //                                                  +MMs                                              
    //                                                 `NMM/                                              
    //                                     `-/++-./-`  /MMM.                                              
    //                                   `+mMMMMMNMMy` sMMy                                               
    //                                   yMMMMMMMMMMMo-NMN.                                               
    //                                -:/MMMMMMMMMMMMMNMMs                                                
    //                             `.-yMMMMMMMMMMMMMMMMMN.                                                
    //                           :yNMMMMMMMMMMMMMMMMMMMMy                                                 
    //                          :MMMMMMMMMMMMMMMMMMMMMMM-                                                 
    //                          `yNNmMMMMMMMMMMMMMMMMMMh                                                  
    //                           `+-yMMMMMMMMMMMMMNMMMM-                                                  
    //                              -NMMMMMMMMMMMMMMMMN.                                                  
    //                              `dNMMMMMMMMMMMMMMMMmy:                                                
    //                               .dMMMMMMMMMMMMMMMMMMN-                                               
    //                              `oMMMNmNMMMMMMMMMMMMMM/                                               
    //                             .hMMMh//mNNMMMMMMMMMMMM+                                               
    //                            -dMMMs` ./-:hMMMMMMMMMMMh                                               
    //                           -mMMMs       `/mMMMMMMMMMM:                                              
    //                           dMMMm/.`       .hMMMMMMMMMm:                                             
    //                          `dNNMMMNho/-.``  /MMMMMMMMMMN/                                            
    //                            .-/shmNNMMNmhhhmMMMMMMMMMMMM+`                                          
    //                                  `-o+oymNNMMMMMMMMMMMMMMh`                                         
    //                                    `  `..-mMMMMMMMMMMMMMMm.                                        
    //                                           yMMMMMMMMMMMMMMMm.                                       
    //                                           hMMMMMMMMMMMMMMMMd                                       
    //                                          `NMMMMMMMMMMMMMMMMM+                                      
    //                                          /MMMMMMMMMMMMMMMMMMd                                      
    //                                          dMMMMMMMMMMMMMMMMMMM-                                     
    //                                         -MMMMMMMMmyysmMMMMMMMo                                     
    //                                         oMMMMMMMN-   :MMMMMMMd                                     
    //                                         hMMMMMMM:     hMMMMMMM`                                    
    //                                         mMMMMMMs      .NMMMMMM/                                    
    //                                        `MMMMMMm`       yMMMMMMs                                    
    //                                        -MMMMMMo        -MMMMMMh                                    
    //                                        :MMMMMN`         yMMMMMm                                    
    //                                        /MMMMMs          -MMMMMN`                                   
    //                                        oMMMMN.           yMMMMM.                                   
    //                                        dMMMMo            -NMMMMs                                   
    //                                       :MMMMs              /NMMMm                                   
    //                                      .mMMMh                /MMMM/                                  
    //                                     `dMMMM/                `MMMMN.                                 
    //                                     sMMMMN`                 mMMMMh                                 
    //                                    .MMMMM+                  oMMMMM:                                
    //                                    oMMMMy                   `hMMMMh                                
    //                                    dMMMy`                    `hMMMM.                               
    //                                   `NMMm`                      `hMMMs                               
    //                                   -MMM/                        .mMMN`                              
    //                                   :MMh                          :MMM+                              
    //                                   oMN-                           yMMd                              
    //                                   dMh                            :MMM:                             
    //                                  :MMo                            .MMMy                             
    //                                  hMM-                            :MMMN`                            
    //                                 :MMm                             .mmMM+                            
    //                                `mMMo                              o.dMh`                           
    //                                sMMN-                              o -NMho`                         
    //                                -::-                               -  :///.                         

    // getContractAddress
    // returns address to smart contract
    //
    // “Lift ’em high and drain ’em dry, to the guy who says, “My turn to buy.”
    //
    pub fun getContractAddress(): Address {
        return self.account.address
    }

    // getBeerPrice
    //
    // "May we never go to hell but always be on our way."
    //
    pub fun getBeerPrice(): UFix64 {
        return self.beerPrice
    }

    // getWhiskyPrice
    //
    // "God in goodness sent us grapes to cheer both great and small. 
    //  Little fools drink too much, and great fools not at all!"
    //
    pub fun getWhiskyPrice(): UFix64 {
        return self.whiskyPrice
    }

    // getLongDrinkPrice
    //
    // "The past is history, the future is a mystery, but today is a gift, 
    //  because it’s the present."
    //
    pub fun getLongDrinkPrice(): UFix64 {
        return self.longDrinkPrice
    }

    // rarityToString
    //
    // "Life is short, but sweet."
    //
    pub fun rarityToString(rarity: UInt64): String {
        switch rarity {
            case 0 as UInt64:
                return "Common"
            case 1 as UInt64:
                return "Rare"
            case 2 as UInt64:
                return "Epic"
            case 3 as UInt64:
                return "Legendary"
        }

        return ""
    }

    // Init function of the smart contract
    //
    // "We drink to those who love us, we drink to those who don’t. 
    //  We drink to those who fuck us, and fuck those who don’t!"
    //
    init() {
        // Initialize the total supply
        self.totalSupply = 0

        // position to take the drinks from
        self.firstIndex = 0

        // Set last call
        self.lastCall = false

        // Initialize drink prices
        self.beerPrice = 4.20
        self.whiskyPrice = 12.60
        self.longDrinkPrice = 42.00

        // Init collections
        self.CollectionStoragePath = /storage/PartyMansionDrinkCollection
        self.CollectionPublicPath = /public/PartyMansionDrinkCollectionPublic

        // init & save Barkeeper, assign to Bar 
        self.BarkeeperStoragePath = /storage/PartyMansionBarkeeper
        self.account.save<@Barkeeper>(<- create Barkeeper(), to: self.BarkeeperStoragePath)

        // Init the Bar
        self.fridge = []
        self.shaker = []
        self.whiskyCellar = []
    }
}