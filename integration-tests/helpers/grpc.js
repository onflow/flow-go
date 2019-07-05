const promisify = require('util').promisify;
const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');

const PING_PROTO_PATH = 'services/ping/ping.proto';
const COLLECT_PROTO_PATH = 'services/collect/collect.proto';
const CONSENSUS_PROTO_PATH = 'services/consensus/consensus.proto';
const EXECUTE_PROTO_PATH = 'services/execute/execute.proto';
const VERIFY_PROTO_PATH = 'services/verify/verify.proto';
const SEAL_PROTO_PATH = 'services/seal/seal.proto';

const options = {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: ['/home/node/app/node_modules/google-proto-files', '/home/node/app/proto']
}

const pingProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(PING_PROTO_PATH, options));
const collectProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(COLLECT_PROTO_PATH, options));
const consensusProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(CONSENSUS_PROTO_PATH, options));
const executeProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(EXECUTE_PROTO_PATH, options));
const verifyProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(VERIFY_PROTO_PATH, options));
const sealProtoDescriptor = grpc.loadPackageDefinition(protoLoader.loadSync(SEAL_PROTO_PATH, options));

function createNewPingStub(address) {
  const stub = new pingProtoDescriptor.bamboo.services.ping.PingService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewCollectStub(address) {
  const stub = new collectProtoDescriptor.bamboo.services.collect.CollectService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewConsensusStub(address) {
  const stub = new consensusProtoDescriptor.bamboo.services.consensus.ConsensusService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewExecuteStub(address) {
  const stub = new executeProtoDescriptor.bamboo.services.execute.ExecuteService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewVerifyStub(address) {
  const stub = new verifyProtoDescriptor.bamboo.services.verify.VerifyService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewSealStub(address) {
  const stub = new sealProtoDescriptor.bamboo.services.seal.SealService(address, grpc.credentials.createInsecure());
  return promisifyStub(stub);
}

function createNewAccessNodeStub(address) {
  const pingStub = createNewPingStub(address);
  const collectStub = createNewCollectStub(address);
  const verifyStub = createNewVerifyStub(address);

  return {
    ping: pingStub,
    collect: collectStub,
    verify: verifyStub,
  }
}

function createNewExecuteNodeStub(address) {
  const pingStub = createNewPingStub(address);
  const executeStub = createNewExecuteStub(address);

  return {
    ping: pingStub,
    execute: executeStub,
  }
}

function createNewSecurityNodeStub(address) {
  const pingStub = createNewPingStub(address);
  const consensusStub = createNewConsensusStub(address);
  const sealStub = createNewSealStub(address);

  return {
    ping: pingStub,
    consensus: consensusStub,
    seal: sealStub,
  }
}

// Note: this is not pretty, but needed because of bind()
function promisifyStub(stub){
  return function(method) {
    return promisify(stub[method]).bind(stub)
  }
}

module.exports = {
  createNewAccessNodeStub,
  createNewSecurityNodeStub,
  createNewExecuteNodeStub,
}
