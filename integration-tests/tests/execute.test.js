const assert = require('assert');
const promisify = require('util').promisify;
const grpcHelpers = require('../helpers/grpc');

const { EXECUTE_ADDR, HELPERS_ADDR} = process.env;

describe('Execute node integration tests', function() {

  beforeEach(function() {
    /* TODO
    return helpersAPI()
      .post('/reset-db')
      .expect(200)
    */

  });

  describe('ping', function() {
    it('should be a able to ping execute', async function() {
      const executeStub = grpcHelpers.createNewExecutionStub(EXECUTE_ADDR) 
      const PingRequest = {};
      const PingResponse = await executeStub("Ping")(PingRequest)
      assert.deepStrictEqual(PingResponse, {address: Buffer.from('ping pong!')})
    });
  });

});
