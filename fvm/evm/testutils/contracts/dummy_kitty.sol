// SPDX-License-Identifier: GPL-3.0

contract DummyKitty {		
    event BirthEvent(address owner, uint256 kittyId, uint256 matronId, uint256 sireId, uint256 genes);
    event TransferEvent(address from, address to, uint256 tokenId);

    struct Kitty {
        uint256 genes;
        uint64 birthTime;
        uint32 matronId;
        uint32 sireId;
        uint16 generation;
    }

    uint256 idCounter;

    // @dev all kitties 
    Kitty[] kitties;

    /// @dev a mapping from cat IDs to the address that owns them. 
    mapping (uint256 => address) public kittyIndexToOwner;

    // @dev a mapping from owner address to count of tokens that address owns.
    mapping (address => uint256) ownershipTokenCount;

    /// @dev a method to transfer kitty
    function Transfer(address _from, address _to, uint256 _tokenId) external {
        // Since the number of kittens is capped to 2^32 we can't overflow this
        ownershipTokenCount[_to]++;
        // transfer ownership
        kittyIndexToOwner[_tokenId] = _to;
        // When creating new kittens _from is 0x0, but we can't account that address.
        if (_from != address(0)) {
            ownershipTokenCount[_from]--;
        }
        // Emit the transfer event.
        emit TransferEvent(_from, _to, _tokenId);
    }

    /// @dev a method callable by anyone to create a kitty
    function CreateKitty(
        uint256 _matronId,
        uint256 _sireId,
        uint256 _generation,
        uint256 _genes
    )
        external
        returns (uint)
    {

        require(_matronId == uint256(uint32(_matronId)));
        require(_sireId == uint256(uint32(_sireId)));
        require(_generation == uint256(uint16(_generation)));

        Kitty memory _kitty = Kitty({
            genes: _genes,
            birthTime: uint64(block.timestamp),
            matronId: uint32(_matronId),
            sireId: uint32(_sireId),
            generation: uint16(_generation)
        });

        kitties.push(_kitty);

        emit BirthEvent(
            msg.sender,
            idCounter,
            uint256(_kitty.matronId),
            uint256(_kitty.sireId),
            _kitty.genes
        );

        this.Transfer(address(0), msg.sender, idCounter);

        idCounter++;

        return idCounter;
    }
}