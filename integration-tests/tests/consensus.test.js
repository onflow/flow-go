const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { CONSENSUS_ADDR, HELPERS_ADDR} = process.env;

describe('Consensus role integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */
  });

  describe('ping', function() {
    it('should be a able to ping', async function() {
      const consensusStub = grpcHelpers.createNewConsensusStub(CONSENSUS_ADDR) 
      const PingRequest = {};
      const PingResponse = await consensusStub("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('pong!')})
    });
  });

});
