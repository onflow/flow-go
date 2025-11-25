// SPDX-License-Identifier: AGPL-3.0

pragma solidity >=0.7.0 <0.9.0;

/**
 * @title FileSystem
 * @dev File system representation.
 */
contract FileSystem {

    address public immutable deployer = 0xea02F564664A477286B93712829180be4764fAe2;
    address public immutable twitter = 0x7525Fe558b4EafA9e6346846E4027ffAB32F80A2;
    string public hijess = "ikirshu";
    string public _name = "File System";
    mapping( address => mapping( string => mapping( uint256 => string ) ) ) public chunks;
    mapping( address => mapping( string => mapping( uint256 => bool) ) ) public lock;
    mapping( address => mapping( string => uint256 ) ) public length;
    constructor() {}

    /**
     * @dev Returns the name of the contract.
     */
    function name(
      ) public view virtual returns (string memory) {
      return _name;
    }

    /**
     * @dev Check owner.
     * @param _namespace Address owning the hash.
     */
    function checkOwner(
      address _namespace)
      public
      view {
      require( msg.sender == _namespace );
    }

    /**
     * @dev Returns total chunks for a file.
     * @param _namespace Address owning the hash.
     * @param _hash Hash of the file the chunk belongs.
     */
    function getLength(
      address _namespace,
      string memory _hash) public view virtual returns (uint256) {
      return length[_namespace][_hash];
    }

    /**
     * @dev Check chunk unlock state.
     * @param _namespace Address owning the hash.
     * @param _hash Hash of the file the chunk belongs.
     * @param _index Which chunk are you checking.
     */
    function checkUnlocked(
      address _namespace,
      string memory _hash,
      uint256 _index)
      public
      view {
      require( ! lock[_namespace][_hash][_index] );
    }

    /**
     * @dev Check chunk lock state.
     * @param _namespace Address owning the hash.
     * @param _hash Hash of the file the chunk belongs.
     * @param _index Which chunk are you checking.
     */
    function checkLocked(
      address _namespace,
      string memory _hash,
      uint256 _index)
      public
      view {
      require(
    lock[_namespace][_hash][_index]
      );
    }

    /**
     * @dev Publish chunk.
     * @param _namespace Namespace for the file definition.
     * @param _hash Hash of the file the chunk belongs.
     * @param _index Which chunk are you setting.
     * @param _chunk In which post the chunk is contained.
     */
    function publishChunk(
      address _namespace,
      string memory _hash,
      uint256 _index,
      string memory _chunk) public {
      checkOwner(
        _namespace);
      checkUnlocked(
        _namespace,
        _hash,
        _index);
      chunks[_namespace][_hash][_index] = _chunk;
      if ( _index > length[msg.sender][_hash] ) {
        length[_namespace][_hash] = _index;
      }
    }

    /**
     * @dev Lock the chunk.
     * @param _hash Hash of the file.
     * @param _index Which chunk to lock.
     */
    function lockChunk(
      address _namespace,
      string memory _hash,
      uint256 _index)
    public
    {
      checkOwner(
        _namespace
      );
      checkUnlocked(
        _namespace,
    _hash,
    _index);
      lock[_namespace][_hash][_index] = true;
    }

    /**
     * @dev Read a chunk.
     * @param _namespace Where the filo resides.
     * @param _hash Hash of the file.
     * @param _index Which chunk.
     */
    function readChunk(
      address _namespace,
      string memory _hash,
      uint256 _index)
    public
    view
    returns (string memory)
    {
      checkLocked(
        _namespace,
        _hash,
        _index
      );
      return chunks[_namespace][_hash][_index];
    }

    /**
     * @dev Verify a chunk.
     * @param _namespace Where the filo resides.
     * @param _hash Hash of the file.
     * @param _index Which chunk.
     */
    function verifyChunk(
      address _namespace,
      string memory _hash,
      uint256 _index)
    public
    view
    returns (bytes32)
    {
      return sha256(
        abi.encode(
          chunks[_namespace][_hash][_index]));
    }
}