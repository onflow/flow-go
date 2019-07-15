const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { SEAL_ADDR, HELPERS_ADDR} = process.env;

describe('Seal role integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */
  });

  describe('ping', function() {
    it('should be a able to ping', async function() {
      const sealStub = grpcHelpers.createNewSealStub(SEAL_ADDR) 
      const PingRequest = {};
      const PingResponse = await sealStub("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('pong!')})
    });
  });

});
