#include "bls_include.h"

// this file is about the core functions required by the BLS signature scheme

// Compute a BLS signature from a G1 point (not checked) and writes it in `out`.
// `out` must be allocated properly with `G1_SER_BYTES` bytes.
static void bls_sign_E1(byte *out, const Fr *sk, const E1 *h) {
  // s = h^sk
  E1 s;
  E1_mult(&s, h, sk);
  E1_write_bytes(out, &s);
}

// Computes a BLS signature from a hash and writes it in `out`.
// `hash` represents the hashed message with length `hash_len` equal to
// `MAP_TO_G1_INPUT_LEN`.
// `out` must be allocated properly with `G1_SER_BYTES` bytes.
int bls_sign(byte *out, const Fr *sk, const byte *hash, const int hash_len) {
  // hash to G1
  E1 h;
  if (map_to_G1(&h, hash, hash_len) != VALID) {
    return INVALID;
  }
  // s = h^sk
  bls_sign_E1(out, sk, &h);
  return VALID;
}

extern const E2 *BLS12_381_minus_g2;

// Verifies a BLS signature (G1 point) against a public key (G2 point)
// and a message hash `h` (G1 point).
// Hash, signature and public key are assumed to be in G1, G1 and G2
// respectively.
// This function only checks the pairing equality.
static int bls_verify_E1(const E2 *pk, const E1 *s, const E1 *h) {
  E1 elemsG1[2];
  E2 elemsG2[2];

  // elemsG1[0] = s, elemsG1[1] = h
  E1_copy(&elemsG1[0], s);
  E1_copy(&elemsG1[1], h);

  // elemsG2[0] = -g2, elemsG2[1] = pk
  E2_copy(&elemsG2[0], BLS12_381_minus_g2);
  E2_copy(&elemsG2[1], pk);

  // double pairing
  Fp12 e;
  Fp12_multi_pairing(&e, elemsG1, elemsG2, 2);
  if (Fp12_is_one(&e)) {
    return VALID;
  }
  return INVALID;
}

// Verifies the validity of an aggregated BLS signature under distinct messages.
//
// Each message is mapped to a set of public keys, so that the verification
// equation is optimized to compute one pairing per message.
// - sig is the signature.
// - nb_hashes is the number of the messages (hashes) in the map
// - hashes is pointer to all flattened hashes in order where the hash at index
// i has a byte length len_hashes[i],
//   is mapped to pks_per_hash[i] public keys.
// - the keys are flattened in pks in the same hashes order.
//
// membership check of the signature in G1 is verified in this function
// membership check of pks in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications
// using the same pks
int bls_verifyPerDistinctMessage(const byte *sig, const int nb_hashes,
                                 const byte *hashes, const uint32_t *len_hashes,
                                 const uint32_t *pks_per_hash, const E2 *pks) {

  int ret = UNDEFINED; // return value

  E1 *elemsG1 = (E1 *)malloc((nb_hashes + 1) * sizeof(E1));
  if (!elemsG1)
    goto outG1;
  E2 *elemsG2 = (E2 *)malloc((nb_hashes + 1) * sizeof(E2));
  if (!elemsG2)
    goto outG2;

  // elemsG1[0] = sig
  if (E1_read_bytes(&elemsG1[0], sig, G1_SER_BYTES) != VALID) {
    ret = INVALID;
    goto out;
  }

  // check signature is in G1
  if (!E1_in_G1(&elemsG1[0])) {
    ret = INVALID;
    goto out;
  }

  // elemsG2[0] = -g2
  E2_copy(&elemsG2[0], BLS12_381_minus_g2);

  // map all hashes to G1
  int offset = 0;
  for (int i = 1; i < nb_hashes + 1; i++) {
    // elemsG1[i] = h
    // hash to G1
    map_to_G1(&elemsG1[i], &hashes[offset], len_hashes[i - 1]);
    offset += len_hashes[i - 1];
  }

  // aggregate public keys mapping to the same hash
  offset = 0;
  for (int i = 1; i < nb_hashes + 1; i++) {
    // elemsG2[i] = agg_pk[i]
    E2_sum_vector(&elemsG2[i], &pks[offset], pks_per_hash[i - 1]);
    offset += pks_per_hash[i - 1];
  }

  // multi pairing
  Fp12 e;
  Fp12_multi_pairing(&e, elemsG1, elemsG2, nb_hashes + 1);
  if (Fp12_is_one(&e)) {
    ret = VALID;
  } else {
    ret = INVALID;
  }

out:
  free(elemsG2);
outG2:
  free(elemsG1);
outG1:
  return ret;
}

// Verifies the validity of an aggregated BLS signature under distinct public
// keys.
//
// Each key is mapped to a set of messages, so that the verification equation is
// optimized to compute one pairing per public key.
// - nb_pks is the number of the public keys in the map.
// - pks is pointer to all pks in order where the key at index i
//   is mapped to hashes_per_pk[i] hashes.
// - the messages (hashes) are flattened in hashes in the same public key order,
//  each with a length in len_hashes.
//
// membership check of the signature in G1 is verified in this function
// membership check of pks in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications
// using the same pks
int bls_verifyPerDistinctKey(const byte *sig, const int nb_pks, const E2 *pks,
                             const uint32_t *hashes_per_pk, const byte *hashes,
                             const uint32_t *len_hashes) {

  int ret = UNDEFINED; // return value

  E1 *elemsG1 = (E1 *)malloc((nb_pks + 1) * sizeof(E1));
  if (!elemsG1)
    goto outG1;
  E2 *elemsG2 = (E2 *)malloc((nb_pks + 1) * sizeof(E2));
  if (!elemsG2)
    goto outG2;

  // elemsG1[0] = s
  if (E1_read_bytes(&elemsG1[0], sig, G1_SER_BYTES) != VALID) {
    ret = INVALID;
    goto out;
  }

  // check s in G1
  if (!E1_in_G1(&elemsG1[0])) {
    ret = INVALID;
    goto out;
  }

  // elemsG2[0] = -g2
  E2_copy(&elemsG2[0], BLS12_381_minus_g2);

  // set the public keys
  for (int i = 1; i < nb_pks + 1; i++) {
    E2_copy(&elemsG2[i], &pks[i - 1]);
  }

  // map all hashes to G1 and aggregate the ones with the same public key

  // tmp_hashes is a temporary array of all hashes under a same key mapped to a
  // G1 point. tmp_hashes size is set to the maximum possible size to minimize
  // malloc calls.
  int tmp_hashes_size = hashes_per_pk[0];
  for (int i = 1; i < nb_pks; i++) {
    if (hashes_per_pk[i] > tmp_hashes_size) {
      tmp_hashes_size = hashes_per_pk[i];
    }
  }
  E1 *tmp_hashes = (E1 *)malloc(tmp_hashes_size * sizeof(E1));
  if (!tmp_hashes) {
    ret = UNDEFINED;
    goto out;
  }

  // sum hashes under the same key
  int data_offset = 0;
  int index_offset = 0;
  for (int i = 1; i < nb_pks + 1; i++) {
    for (int j = 0; j < hashes_per_pk[i - 1]; j++) {
      // map the hash to G1
      map_to_G1(&tmp_hashes[j], &hashes[data_offset], len_hashes[index_offset]);
      data_offset += len_hashes[index_offset];
      index_offset++;
    }
    // aggregate all the points of the array
    E1_sum_vector(&elemsG1[i], tmp_hashes, hashes_per_pk[i - 1]);
  }
  free(tmp_hashes);

  // multi pairing
  Fp12 e;
  Fp12_multi_pairing(&e, elemsG1, elemsG2, nb_pks + 1);

  if (Fp12_is_one(&e)) {
    ret = VALID;
  } else {
    ret = INVALID;
  }

out:
  free(elemsG2);
outG2:
  free(elemsG1);
outG1:
  return ret;
}

// Verifies a BLS signature in a byte buffer.
// membership check of the signature in G1 is verified.
// membership check of pk in G2 is not verified in this function.
// the membership check in G2 is separated to optimize multiple verifications
// using the same key. `hash` represents the hashed message with length
// `hash_len` equal to `MAP_TO_G1_INPUT_LEN`.
int bls_verify(const E2 *pk, const byte *sig, const byte *hash,
               const int hash_len) {
  E1 s, h;
  // deserialize the signature into a curve point
  if (E1_read_bytes(&s, sig, G1_SER_BYTES) != VALID) {
    return INVALID;
  }

  // check s is in G1
  if (!E1_in_G1(&s)) {
    return INVALID;
  }

  if (map_to_G1(&h, hash, hash_len) != VALID) {
    return INVALID;
  }

  return bls_verify_E1(pk, &s, &h);
}

// binary tree structure to be used by bls_batch verify.
// Each node contains a signature and a public key, the signature (resp. the
// public key) being the aggregated signature of the two children's signature
// (resp. public keys). The leaves contain the initial signatures and public
// keys.
typedef struct st_node {
  E1 *sig;
  E2 *pk;
  struct st_node *left;
  struct st_node *right;
} node;

static node *new_node(const E2 *pk, const E1 *sig) {
  node *t = (node *)malloc(sizeof(node));
  if (t) {
    t->pk = (E2 *)pk;
    t->sig = (E1 *)sig;
    t->right = t->left = NULL;
  }
  return t;
}

static void free_tree(node *root) {
  if (!root)
    return;

  // only free pks and sigs of non-leafs, data of leafs are allocated
  // as an entire array in `bls_batch_verify`.
  if (root->left) { // no need to check the right child for the leaf check
                    // because
                    //  the recursive build starts with the left side first
    // pointer free
    free(root->sig);
    free(root->pk);
    // free the children nodes
    free_tree(root->left);
    free_tree(root->right);
  }
  free(root);
}

// builds a binary tree of aggregation of signatures and public keys
// recursively.
static node *build_tree(const int len, const E2 *pks, const E1 *sigs) {
  // check if a leaf is reached
  if (len == 1) {
    return new_node(&pks[0], &sigs[0]); // use the first element of the arrays
  }

  // a leaf is not reached yet,
  int right_len = len / 2;
  int left_len = len - right_len;

  // create a new node with new points
  E2 *new_pk = (E2 *)malloc(sizeof(E2));
  if (!new_pk) {
    goto error;
  }
  E1 *new_sig = (E1 *)malloc(sizeof(E1));
  if (!new_sig) {
    goto error_sig;
  }

  node *t = new_node(new_pk, new_sig);
  if (!t)
    goto error_node;

  // build the tree in a top-down way
  t->left = build_tree(left_len, &pks[0], &sigs[0]);
  if (!t->left) {
    free_tree(t);
    goto error;
  }

  t->right = build_tree(right_len, &pks[left_len], &sigs[left_len]);
  if (!t->right) {
    free_tree(t);
    goto error;
  }
  // sum the children
  E1_add(t->sig, t->left->sig, t->right->sig);
  E2_add(t->pk, t->left->pk, t->right->pk);
  return t;

error_node:
  free(new_sig);
error_sig:
  free(new_pk);
error:
  return NULL;
}

// verify the binary tree and fill the results using recursive batch
// verifications.
static void bls_batch_verify_tree(const node *root, const int len,
                                  byte *results, const E1 *h) {
  // verify the aggregated signature against the aggregated public key.
  int res = bls_verify_E1(root->pk, root->sig, h);

  // if the result is valid, all the subtree signatures are valid.
  if (res == VALID) {
    for (int i = 0; i < len; i++) {
      if (results[i] == UNDEFINED)
        results[i] = VALID; // do not overwrite invalid results
    }
    return;
  }

  // check if root is a leaf
  if (root->left == NULL) { // no need to check the right side
    *results = INVALID;
    return;
  }

  // otherwise, at least one of the subtree signatures is invalid.
  // use the binary tree structure to find the invalid signatures.
  int right_len = len / 2;
  int left_len = len - right_len;
  bls_batch_verify_tree(root->left, left_len, &results[0], h);
  bls_batch_verify_tree(root->right, right_len, &results[left_len], h);
}

// Batch verifies the validity of a multiple BLS signatures of the
// same message under multiple public keys. Each signature at index `i` is
// verified against the public key at index `i`. `seed` is used as the entropy
// source for randoms required by the computation. The function assumes the
// source size is at least (16*sigs_len) of random bytes of entropy at least 128
// bits.
//
// - membership checks of all signatures is verified upfront.
// - use random coefficients for signatures and public keys at the same index to
// prevent
//  indices mixup.
// - optimize the verification by verifying an aggregated signature against an
// aggregated
//  public key, and use a top-down recursive verification to find invalid
//  signatures.
void bls_batch_verify(const int sigs_len, byte *results, const E2 *pks_input,
                      const byte *sigs_bytes, const byte *data,
                      const int data_len, const byte *seed) {

  // initialize results to undefined
  memset(results, UNDEFINED, sigs_len);

  // build the arrays of G1 and G2 elements to verify
  E2 *pks = (E2 *)malloc(sigs_len * sizeof(E2));
  if (!pks) {
    return;
  }
  E1 *sigs = (E1 *)malloc(sigs_len * sizeof(E1));
  if (!sigs) {
    goto out_sigs;
  }

  E1 h;
  if (map_to_G1(&h, data, data_len) != VALID) {
    goto out;
  }

  for (int i = 0; i < sigs_len; i++) {
    // convert the signature points:
    // - invalid points are stored as infinity points with an invalid result, so
    // that the tree aggregations remain valid.
    // - valid points are multiplied by a random scalar (same for public keys at
    // same index) to make sure a signature at index (i) is verified against the
    // public key at the same index.
    int read_ret =
        E1_read_bytes(&sigs[i], &sigs_bytes[G1_SER_BYTES * i], G1_SER_BYTES);
    if (read_ret != VALID || !E1_in_G1(&sigs[i])) {
      // set signature and key to infinity (no effect on the aggregation tree)
      // and set result to invalid (result won't be overwritten)
      E2_set_infty(&pks[i]);
      E1_set_infty(&sigs[i]);
      results[i] = INVALID;
    } else {
      // choose a random non-zero coefficient of at least 128 bits
      Fr r, one;
      // r = random, i-th seed is used for i-th signature
      Fr_set_zero(&r);
      const int seed_len = SEC_BITS / 8;
      limbs_from_be_bytes((limb_t *)&r, seed + (seed_len * i),
                          seed_len); // faster shortcut than Fr_map_bytes
      // r = random + 1
      Fr_set_limb(&one, 1);
      Fr_add(&r, &r, &one);
      // multiply public key and signature by the same random exponent r
      E2_mult(&pks[i], &pks_input[i], &r);
      E1_mult(&sigs[i], &sigs[i], &r);
    }
  }
  // build a binary tree of aggregations
  node *root = build_tree(sigs_len, &pks[0], &sigs[0]);
  if (!root) {
    goto out;
  }

  // verify the binary tree and fill the results using batch verification
  bls_batch_verify_tree(root, sigs_len, &results[0], &h);
  // free the allocated tree
  free_tree(root);
out:
  free(sigs);
out_sigs:
  free(pks);
}

// Verifies the validity of 2 SPoCK proofs and 2 public keys.
// Membership check in G1 of both proofs is verified in this function.
// Membership check in G2 of both keys is not verified in this function.
// the membership check in G2 is separated to allow optimizing multiple
// verifications using the same public keys.
int bls_spock_verify(const E2 *pk1, const byte *sig1, const E2 *pk2,
                     const byte *sig2) {
  E1 elemsG1[2];
  E2 elemsG2[2];

  // elemsG1[0] = s1
  if (E1_read_bytes(&elemsG1[0], sig1, G1_SER_BYTES) != VALID) {
    return INVALID;
  };
  // check s1 is in G1
  if (!E1_in_G1(&elemsG1[0])) {
    return INVALID;
  }

  // elemsG1[1] = s2
  if (E1_read_bytes(&elemsG1[1], sig2, G1_SER_BYTES) != VALID) {
    return INVALID;
  };
  // check s2 is in G1
  if (!E1_in_G1(&elemsG1[1])) {
    return INVALID;
  }

  // elemsG2[1] = pk1
  E2_copy(&elemsG2[1], pk1);

  // elemsG2[0] = -pk2
  E2_neg(&elemsG2[0], pk2);

  // double pairing
  Fp12 e;
  Fp12_multi_pairing(&e, elemsG1, elemsG2, 2);

  if (Fp12_is_one(&e)) {
    return VALID;
  }
  return INVALID;
}
