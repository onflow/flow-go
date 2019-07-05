const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { SECURITY_ADDR, HELPERS_ADDR } = process.env;

describe('Security node integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */

  });

  describe('ping', function() {
    it('should be a able to ping security', async function() {
      const securityStub = grpcHelpers.createNewSecurityNodeStub(SECURITY_ADDR) 
      const PingRequest = {};
      const PingResponse = await securityStub.ping("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('pong!')})
    });
  });


});
