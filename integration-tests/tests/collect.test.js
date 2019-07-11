const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { COLLECT_ADDR, HELPERS_ADDR} = process.env;

describe('Collect role integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */
  });

  describe('ping', function() {
    it('should be a able to ping', async function() {
      const collectStub = grpcHelpers.createNewCollectStub(COLLECT_ADDR) 
      const PingRequest = {};
      const PingResponse = await collectStub("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('pong!')})
    });
  });

});
