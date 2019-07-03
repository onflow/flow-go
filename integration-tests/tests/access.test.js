const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { ACCESS_ADDR, HELPERS_ADDR} = process.env;

describe('Access node integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */

  });

  describe('ping', function() {
    it('should be a able to ping access', async function() {
      const accessStub = grpcHelpers.createNewAccessStub(ACCESS_ADDR) 
      const PingRequest = {};
      const PingResponse = await accessStub("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('ping pong!')})
    });
  });

});
