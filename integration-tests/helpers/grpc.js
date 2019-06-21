const promisify = require('util').promisify;
const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');

const EXECUTE_PROTO_PATH = 'inter/execute.proto';
const SECURITY_PROTO_PATH = 'inter/security.proto';
const options = {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: ['/home/node/app/node_modules/google-proto-files', '/home/node/app/proto']
}
const executeProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(EXECUTE_PROTO_PATH, options));
const securityProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(SECURITY_PROTO_PATH, options));


function createNewExecutionStub(address) {
  const stub = new executeProtoDescriptor.bamboo.proto.ExecuteNode(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewSecurityStub(address) {
  const stub = new securityProtoDescriptor.bamboo.proto.SecurityNode(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

// Note: this is not pretty, but needed because of bind()
function promisifyStub(stub){
  return function(method) {
    return promisify(stub[method]).bind(stub)
  }
}

module.exports = {
  createNewExecutionStub,
  createNewSecurityStub,
}
