// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold
// signature, BLS-SPoCK and the BLS distributed key generation protocols

#include "bls12381_utils.h"
#include "assert.h"
#include "bls_include.h"

// compile all blst C src along with this file
#include "blst_src.c"

// make sure flow crypto types are consistent with BLST types
void types_sanity(void) {
  assert(sizeof(Fr) == sizeof(vec256));
  assert(sizeof(Fp) == sizeof(vec384));
  assert(sizeof(Fp2) == sizeof(vec384x));
  assert(sizeof(E1) == sizeof(POINTonE1));
  assert(sizeof(E2) == sizeof(POINTonE2));
  assert(sizeof(Fp12) == sizeof(vec384fp12));
}

// ------------------- Fr utilities

// Montgomery constant R related to the curve order r
// R = (1<<256) mod r
const Fr BLS12_381_rR = {{
    TO_LIMB_T(0x1824b159acc5056f),
    TO_LIMB_T(0x998c4fefecbc4ff5),
    TO_LIMB_T(0x5884b7fa00034802),
    TO_LIMB_T(0x00000001fffffffe),
}};

// returns true if a is zero and false otherwise
bool Fr_is_zero(const Fr *a) { return vec_is_zero(a, sizeof(Fr)); }

// returns true if a == b and false otherwise
bool Fr_is_equal(const Fr *a, const Fr *b) {
  return vec_is_equal(a, b, sizeof(Fr));
}

// sets `a` to limb `l`
void Fr_set_limb(Fr *a, const limb_t l) {
  vec_zero((byte *)a + sizeof(limb_t), sizeof(Fr) - sizeof(limb_t));
  *((limb_t *)a) = l;
}

void Fr_copy(Fr *res, const Fr *a) {
  if ((uptr_t)a == (uptr_t)res) {
    return;
  }
  vec_copy((byte *)res, (byte *)a, sizeof(Fr));
}

// sets `a` to 0
void Fr_set_zero(Fr *a) { vec_zero((byte *)a, sizeof(Fr)); }

void Fr_add(Fr *res, const Fr *a, const Fr *b) {
  add_mod_256((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_r);
}

void Fr_sub(Fr *res, const Fr *a, const Fr *b) {
  sub_mod_256((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_r);
}

void Fr_neg(Fr *res, const Fr *a) {
  cneg_mod_256((limb_t *)res, (limb_t *)a, 1, BLS12_381_r);
}

// res = a*b*R^(-1)
void Fr_mul_montg(Fr *res, const Fr *a, const Fr *b) {
  mul_mont_sparse_256((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_r, r0);
}

// res = a^2 * R^(-1)
void Fr_squ_montg(Fr *res, const Fr *a) {
  sqr_mont_sparse_256((limb_t *)res, (limb_t *)a, BLS12_381_r, r0);
}

// res = a*R
void Fr_to_montg(Fr *res, const Fr *a) {
  mul_mont_sparse_256((limb_t *)res, (limb_t *)a, BLS12_381_rRR, BLS12_381_r,
                      r0);
}

// res = a*R^(-1)
void Fr_from_montg(Fr *res, const Fr *a) {
  from_mont_256((limb_t *)res, (limb_t *)a, BLS12_381_r, r0);
}

// res = a^(-1)*R
void Fr_inv_montg_eucl(Fr *res, const Fr *a) {
  // copied and modified from BLST code
  // Copyright Supranational LLC
  static const vec256 rx2 = {
      /* left-aligned value of the modulus */
      TO_LIMB_T(0xfffffffe00000002),
      TO_LIMB_T(0xa77b4805fffcb7fd),
      TO_LIMB_T(0x6673b0101343b00a),
      TO_LIMB_T(0xe7db4ea6533afa90),
  };
  vec512 temp;
  ct_inverse_mod_256(temp, (limb_t *)a, BLS12_381_r, rx2);
  redc_mont_256((limb_t *)res, temp, BLS12_381_r, r0);
}

// computes the sum of the array elements and writes the sum in jointx
void Fr_sum_vector(Fr *jointx, const Fr x[], const int x_len) {
  Fr_set_zero(jointx);
  for (int i = 0; i < x_len; i++) {
    Fr_add(jointx, jointx, &x[i]);
  }
}

// internal type of BLST `pow256` uses bytes little endian.
// input is bytes big endian as used by Flow crypto lib external scalars.
static void pow256_from_be_bytes(pow256 ret, const byte a[Fr_BYTES]) {
  byte *b = (byte *)a + Fr_BYTES - 1;
  if ((uptr_t)ret == (uptr_t)a) { // swap in place
    for (int i = 0; i < Fr_BYTES / 2; i++) {
      byte tmp = *ret;
      *(ret++) = *b;
      *(b--) = tmp;
    }
  } else {
    for (int i = 0; i < Fr_BYTES; i++) {
      *(ret++) = *(b--);
    }
  }
}

// internal type of BLST `pow256` uses bytes little endian.
static void pow256_from_Fr(pow256 ret, const Fr *in) {
  le_bytes_from_limbs(ret, (limb_t *)in, Fr_BYTES);
}

// reads a scalar in `a` and checks it is a valid Fr element (a < r).
// input is bytes-big-endian.
// returns:
//    - BAD_ENCODING if the length is invalid
//    - BAD_VALUE if the scalar isn't in Fr
//    - VALID if the scalar is valid
ERROR Fr_read_bytes(Fr *a, const byte *in, int in_len) {
  if (in_len != Fr_BYTES) {
    return BAD_ENCODING;
  }
  // compare to r using BLST internal function
  pow256 tmp;
  pow256_from_be_bytes(tmp, in);
  // (check_mod_256 compares pow256 against a vec256!)
  if (!check_mod_256(tmp, BLS12_381_r)) {
    return BAD_VALUE;
  }
  vec_zero(tmp, sizeof(tmp));
  limbs_from_be_bytes((limb_t *)a, in, Fr_BYTES);
  return VALID;
}

// reads a scalar in `a` and checks it is a valid Fr_star element (0 < a < r).
// input bytes are big endian.
// returns:
//    - BAD_ENCODING if the length is invalid
//    - BAD_VALUE if the scalar isn't in Fr_star
//    - VALID if the scalar is valid
ERROR Fr_star_read_bytes(Fr *a, const byte *in, int in_len) {
  int ret = Fr_read_bytes(a, in, in_len);
  if (ret != VALID) {
    return ret;
  }
  // check if a=0
  if (Fr_is_zero(a)) {
    return BAD_VALUE;
  }
  return VALID;
}

// write Fr element `a` in big endian bytes.
void Fr_write_bytes(byte *out, const Fr *a) {
  // be_bytes_from_limbs works for both limb endianness types
  be_bytes_from_limbs(out, (limb_t *)a, Fr_BYTES);
}

// maps big-endian bytes of any size into an Fr element using modular reduction.
// Input is byte-big-endian, output is Fr (internally vec256).
//
// Note: could use redc_mont_256(vec256 ret, const vec512 a, const vec256 p,
// limb_t n0) to reduce 512 bits at a time.
static void Fr_from_be_bytes(Fr *out, const byte *in, const int in_len) {
  // input can be written in base 2^|R|, with R the Montgomery constant
  // N = l_1 + L_2*2^|R| .. + L_n*2^(|R|*(n-1))
  // Therefore N mod p can be expressed using R as:
  // N mod p = l_1 + L_2*R .. + L_n*R^(n-1)
  Fr digit, radix;
  Fr_set_zero(out);
  Fr_copy(&radix, (Fr *)BLS12_381_rRR); // R^2

  int n = in_len;
  byte *p = (byte *)in + in_len;
  while (n > Fr_BYTES) {
    // limbs_from_be_bytes works for both limb endiannesses
    limbs_from_be_bytes((limb_t *)&digit, p -= Fr_BYTES, Fr_BYTES); // l_i
    Fr_mul_montg(&digit, &digit,
                 &radix); // l_i * R^i  (i is the loop number starting at 1)
    Fr_add(out, out, &digit);
    Fr_mul_montg(&radix, &radix, (Fr *)BLS12_381_rRR); // R^(i+1)
    n -= Fr_BYTES;
  }
  Fr_set_zero(&digit);
  limbs_from_be_bytes((limb_t *)&digit, p - n, n);
  Fr_mul_montg(&digit, &digit, &radix);
  Fr_add(out, out, &digit);
  // at this point : out = l_1*R + L_2*R^2 .. + L_n*R^n,
  // reduce the extra R
  Fr_from_montg(out, out);
  // clean up possible sensitive data
  Fr_set_zero(&digit);
}

// Reads a scalar from an array and maps it to Fr using modular reduction.
// Input is byte-big-endian as used by the external APIs.
// It returns true if scalar is zero and false otherwise.
bool map_bytes_to_Fr(Fr *a, const byte *in, int in_len) {
  Fr_from_be_bytes(a, in, in_len);
  return Fr_is_zero(a);
}

// ------------------- Fp utilities

// Montgomery constants related to the prime p
const Fp BLS12_381_pR = {ONE_MONT_P}; /* R mod p = (1<<384)%p */

// sets `a` to 0
static void Fp_set_zero(Fp *a) { vec_zero((byte *)a, sizeof(Fp)); }

// sets `a` to limb `l`
static void Fp_set_limb(Fp *a, const limb_t l) {
  vec_zero((byte *)a + sizeof(limb_t), sizeof(Fp) - sizeof(limb_t));
  *((limb_t *)a) = l;
}

void Fp_copy(Fp *res, const Fp *a) {
  if ((uptr_t)a == (uptr_t)res) {
    return;
  }
  vec_copy((byte *)res, (byte *)a, sizeof(Fp));
}

static void Fp_add(Fp *res, const Fp *a, const Fp *b) {
  add_mod_384((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_P);
}

static void Fp_sub(Fp *res, const Fp *a, const Fp *b) {
  sub_mod_384((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_P);
}

static void Fp_neg(Fp *res, const Fp *a) {
  cneg_mod_384((limb_t *)res, (limb_t *)a, 1, BLS12_381_P);
}

// checks if `a` is a quadratic residue in Fp. If yes, it computes
// the square root in `res`.
//
// The boolean output is valid whether `a` is in Montgomery form or not,
// since montgomery constant `R` is a quadratic residue.
// However, the square root is valid only if `a` is in montgomery form.
static bool Fp_sqrt_montg(Fp *res, const Fp *a) {
  return sqrt_fp((limb_t *)res, (limb_t *)a);
}

static bool Fp_check(const Fp *a) {
  // use same method as in BLST internal function
  // which seems the most efficient. The method uses the assembly-based
  // modular addition instead of limbs comparison
  Fp temp;
  Fp_add(&temp, a, &ZERO_384);
  return vec_is_equal(&temp, a, Fp_BYTES);
  // no need to clear `tmp` as no current use-case involves sensitive data being
  // passed as `a`
}

// res = a*b*R^(-1)
void Fp_mul_montg(Fp *res, const Fp *a, const Fp *b) {
  mul_mont_384((limb_t *)res, (limb_t *)a, (limb_t *)b, BLS12_381_P, p0);
}

// res = a^2 * R^(-1)
void Fp_squ_montg(Fp *res, const Fp *a) {
  sqr_mont_384((limb_t *)res, (limb_t *)a, BLS12_381_P, p0);
}

// res = a*R
void Fp_to_montg(Fp *res, const Fp *a) {
  mul_mont_384((limb_t *)res, (limb_t *)a, BLS12_381_RR, BLS12_381_P, p0);
}

// res = a*R^(-1)
void Fp_from_montg(Fp *res, const Fp *a) {
  from_mont_384((limb_t *)res, (limb_t *)a, BLS12_381_P, p0);
}

// reads a scalar in `out` and checks it is a valid Fp element (out < p).
// input is bytes-big-endian.
// returns:
//    - BAD_ENCODING if the length is invalid
//    - BAD_VALUE if the scalar isn't in Fp
//    - VALID if the scalar is valid
ERROR Fp_read_bytes(Fp *out, const byte *in, int in_len) {
  if (in_len != Fp_BYTES) {
    return BAD_ENCODING;
  }
  limbs_from_be_bytes((limb_t *)out, in, Fp_BYTES);
  // compare read scalar to p
  if (!Fp_check(out)) {
    return BAD_VALUE;
  }
  return VALID;
}

// write Fp element to `out`,
// assuming `out` has  `Fp_BYTES` allocated bytes.
void Fp_write_bytes(byte *out, const Fp *a) {
  be_bytes_from_limbs(out, (limb_t *)a, Fp_BYTES);
}

// returns the sign of y:
// 1 if y > (p - 1)/2 and 0 otherwise.
// y is in montgomery form!
static byte Fp_get_sign(const Fp *y) {
  // - BLST's sgn0_pty_mont_384 requires input to be in Montg form.
  // - The needed sign bit is on position 1
  return (sgn0_pty_mont_384((const limb_t *)y, BLS12_381_P, p0) >> 1) & 1;
}

// ------------------- Fp^2 utilities

// sets `a` to limb `l`
static void Fp2_set_limb(Fp2 *a, const limb_t l) {
  Fp_set_limb(&real(a), l);
  Fp_set_zero(&imag(a));
}

static void Fp2_add(Fp2 *res, const Fp2 *a, const Fp2 *b) {
  add_mod_384x((vec384 *)res, (vec384 *)a, (vec384 *)b, BLS12_381_P);
}

static void Fp2_sub(Fp2 *res, const Fp2 *a, const Fp2 *b) {
  sub_mod_384x((vec384 *)res, (vec384 *)a, (vec384 *)b, BLS12_381_P);
}

static void Fp2_neg(Fp2 *res, const Fp2 *a) {
  cneg_mod_384(real(res), real(a), 1, BLS12_381_P);
  cneg_mod_384(imag(res), imag(a), 1, BLS12_381_P);
}

// res = a*b in montgomery form
static void Fp2_mul_montg(Fp2 *res, const Fp2 *a, const Fp2 *b) {
  mul_mont_384x((vec384 *)res, (vec384 *)a, (vec384 *)b, BLS12_381_P, p0);
}

// res = a^2 in montgomery form
static void Fp2_squ_montg(Fp2 *res, const Fp2 *a) {
  sqr_mont_384x((vec384 *)res, (vec384 *)a, BLS12_381_P, p0);
}

// checks if `a` is a quadratic residue in Fp^2. If yes, it computes
// the square root in `res`.
//
// The boolean output is valid whether `a` is in Montgomery form or not,
// since montgomery constant `R` is itself a quadratic residue.
// However, the square root is correct only if `a` is in montgomery form
// (the square root would be in montgomery form too).
static bool Fp2_sqrt_montg(Fp2 *res, const Fp2 *a) {
  return sqrt_fp2((vec384 *)res, (vec384 *)a);
}

// returns the sign of y:
// sign(y_0) if y_1 = 0, else sign(y_1).
// y coordinates must be in montgomery form!
static byte Fp2_get_sign(Fp2 *y) {
  // - BLST's sgn0_pty_mont_384x requires input to be in montgomery form.
  // - the sign bit is on position 1
  return (sgn0_pty_mont_384x((vec384 *)y, BLS12_381_P, p0) >> 1) & 1;
}

// reads an Fp^2 element in `a`.
// input is a serialization of real(a) concatenated to serializetion of imag(a).
// a[i] are both Fp elements.
// returns:
//    - BAD_ENCODING if the length is invalid
//    - BAD_VALUE if the scalar isn't in Fp
//    - VALID if the scalar is valid
static ERROR Fp2_read_bytes(Fp2 *a, const byte *in, int in_len) {
  if (in_len != Fp2_BYTES) {
    return BAD_ENCODING;
  }
  ERROR ret = Fp_read_bytes(&real(a), in, Fp_BYTES);
  if (ret != VALID) {
    return ret;
  }
  ret = Fp_read_bytes(&imag(a), in + Fp_BYTES, Fp_BYTES);
  if (ret != VALID) {
    return ret;
  }
  return VALID;
}

// write Fp2 element to bin and assume `bin` has `Fp2_BYTES` allocated bytes.
void Fp2_write_bytes(byte *out, const Fp2 *a) {
  Fp_write_bytes(out, &real(a));
  Fp_write_bytes(out + Fp_BYTES, &imag(a));
}

// ------------------- E1 utilities

void E1_copy(E1 *res, const E1 *p) {
  if ((uptr_t)p == (uptr_t)res) {
    return;
  }
  vec_copy(res, p, sizeof(E1));
}

// checks p1 == p2
bool E1_is_equal(const E1 *p1, const E1 *p2) {
  // `POINTonE1_is_equal` includes the infinity case
  return POINTonE1_is_equal((const POINTonE1 *)p1, (const POINTonE1 *)p2);
}

// compare `p` to infinity
bool E1_is_infty(const E1 *p) {
  // BLST infinity points are defined by Z=0
  return vec_is_zero(p->z, sizeof(p->z));
}

// set `p` to infinity
void E1_set_infty(E1 *p) {
  // BLST infinity points are defined by Z=0
  vec_zero(p->z, sizeof(p->z));
}

// converts an E1 point from Jacobian into affine coordinates (z=1)
void E1_to_affine(E1 *res, const E1 *p) {
  // optimize in case coordinates are already affine
  if (vec_is_equal(p->z, BLS12_381_pR, Fp_BYTES)) {
    E1_copy(res, p);
    return;
  }
  // convert from Jacobian
  POINTonE1_from_Jacobian((POINTonE1 *)res, (const POINTonE1 *)p);
}

// checks affine point `p` is in E1
bool E1_affine_on_curve(const E1 *p) {
  // BLST's `POINTonE1_affine_on_curve` does not include the infinity case!
  return POINTonE1_affine_on_curve((POINTonE1_affine *)p) | E1_is_infty(p);
}

// checks if input E1 point is on the subgroup G1.
// It assumes input `p` is on E1.
bool E1_in_G1(const E1 *p) {
  // currently uses Scott method
  return POINTonE1_in_G1((const POINTonE1 *)p);
}

// E1_read_bytes imports a E1(Fp) point from a buffer in a compressed or
// uncompressed form. The resulting point is guaranteed to be on curve E1 (no G1
// check is included). Expected serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
//
// returns:
//    - BAD_ENCODING if the length is invalid or serialization header bits are
//    invalid
//    - BAD_VALUE if Fp coordinates couldn't deserialize
//    - POINT_NOT_ON_CURVE if deserialized point isn't on E1
//    - VALID if deserialization is valid

// Note: could use POINTonE1_Deserialize_BE and POINTonE1_Uncompress_Z,
//       but needs to update the logic around G2 subgroup check
ERROR E1_read_bytes(E1 *a, const byte *in, const int in_len) {
  // check the length
  if (in_len != G1_SER_BYTES) {
    return BAD_ENCODING;
  }

  // check the compression bit
  int compressed = in[0] >> 7;
  if ((compressed == 1) != (G1_SERIALIZATION == COMPRESSED)) {
    return BAD_ENCODING;
  }

  // check if the point in infinity
  int is_infinity = in[0] & 0x40;
  if (is_infinity) {
    // the remaining bits need to be cleared
    if (in[0] & 0x3F) {
      return BAD_ENCODING;
    }
    for (int i = 1; i < G1_SER_BYTES - 1; i++) {
      if (in[i]) {
        return BAD_ENCODING;
      }
    }
    E1_set_infty(a);
    return VALID;
  }

  // read the sign bit and check for consistency
  int y_sign = (in[0] >> 5) & 1;
  if (y_sign && (!compressed)) {
    return BAD_ENCODING;
  }

  // use a temporary buffer to mask the header bits and read a.x
  byte temp[Fp_BYTES];
  memcpy(temp, in, Fp_BYTES);
  temp[0] &= 0x1F; // clear the header bits
  ERROR ret = Fp_read_bytes(&a->x, temp, sizeof(temp));
  if (ret != VALID) {
    return ret;
  }
  Fp_to_montg(&a->x, &a->x);

  // set a.z to 1
  Fp_copy(&a->z, &BLS12_381_pR);

  if (G1_SERIALIZATION == UNCOMPRESSED) {
    ret = Fp_read_bytes(&a->y, in + Fp_BYTES, sizeof(a->y));
    if (ret != VALID) {
      return ret;
    }
    Fp_to_montg(&a->y, &a->y);
    // check read point is on curve
    if (!E1_affine_on_curve(a)) {
      return POINT_NOT_ON_CURVE;
    }
    return VALID;
  }

  // compute the possible square root
  Fp_squ_montg(&a->y, &a->x);
  Fp_mul_montg(&a->y, &a->y, &a->x); // x^3
  Fp_add(&a->y, &a->y, &B_E1);       // B_E1 is already in montg form
  // check whether x^3+b is a quadratic residue
  if (!Fp_sqrt_montg(&a->y, &a->y)) {
    return POINT_NOT_ON_CURVE;
  }

  // resulting (x,y) is guaranteed to be on curve (y is already in montg form)
  if (Fp_get_sign(&a->y) != y_sign) {
    Fp_neg(&a->y, &a->y); // flip y sign if needed
  }
  return VALID;
}

// E1_write_bytes exports a point in E1(Fp) to a buffer in a compressed or
// uncompressed form. It assumes buffer is of length G1_SER_BYTES The
// serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
void E1_write_bytes(byte *out, const E1 *a) {
  if (E1_is_infty(a)) {
    memset(out, 0, G1_SER_BYTES);
    // set the infinity bit
    out[0] = (G1_SERIALIZATION << 7) | (1 << 6);
    return;
  }
  E1 tmp;
  E1_to_affine(&tmp, a);

  Fp_from_montg(&tmp.x, &tmp.x);
  Fp_write_bytes(out, &tmp.x);

  if (G1_SERIALIZATION == COMPRESSED) {
    out[0] |= (Fp_get_sign(&tmp.y) << 5);
  } else {
    Fp_from_montg(&tmp.y, &tmp.y);
    Fp_write_bytes(out + Fp_BYTES, &tmp.y);
  }
  // compression bit
  out[0] |= (G1_SERIALIZATION << 7);
}

// generic point addition that must handle doubling and points at infinity
void E1_add(E1 *res, const E1 *a, const E1 *b) {
  POINTonE1_dadd((POINTonE1 *)res, (POINTonE1 *)a, (POINTonE1 *)b, NULL);
}

// Point negation: res = -a
void E1_neg(E1 *res, const E1 *a) {
  E1_copy(res, a);
  POINTonE1_cneg((POINTonE1 *)res, 1);
}

// Exponentiation of a generic point `a` in E1, res = expo.a
void E1_mult(E1 *res, const E1 *p, const Fr *expo) {
  pow256 tmp;
  pow256_from_Fr(tmp, expo);
  POINTonE1_mult_glv((POINTonE1 *)res, (POINTonE1 *)p, tmp);
  vec_zero(&tmp, sizeof(tmp));
}

// computes the sum of the E1 array elements `y[i]` and writes it in `sum`.
void E1_sum_vector(E1 *sum, const E1 *y, const int len) {
  E1_set_infty(sum);
  for (int i = 0; i < len; i++) {
    E1_add(sum, sum, &y[i]);
  }
}

// Computes the sum of input E1 elements flattened in a single byte
// array `in_bytes` of `in_len` bytes. and writes the sum (E1 element) as
// bytes in `out`.
// The function does not check membership of E1 inputs in G1
// subgroup. The header is using byte pointers to minimize Cgo calls from the Go
// layer.
int E1_sum_vector_byte(byte *out, const byte *in_bytes, const int in_len) {
  int error = UNDEFINED;
  // sanity check that `len` is multiple of `G1_SER_BYTES`
  if (in_len % G1_SER_BYTES) {
    error = INVALID;
    goto mem_error;
  }
  int n = in_len / G1_SER_BYTES; // number of signatures

  E1 *vec = (E1 *)malloc(n * sizeof(E1));
  if (!vec) {
    goto mem_error;
  }

  // import the points from the array
  for (int i = 0; i < n; i++) {
    // deserialize each point from the input array
    if (E1_read_bytes(&vec[i], &in_bytes[G1_SER_BYTES * i], G1_SER_BYTES) !=
        VALID) {
      error = INVALID;
      goto out;
    }
  }
  // sum the points
  E1 acc;
  E1_sum_vector(&acc, vec, n);
  // export the result
  E1_write_bytes(out, &acc);
  error = VALID;
out:
  free(vec);
mem_error:
  return error;
}

// Exponentiation of generator g1 of G1, res = expo.g1
void G1_mult_gen(E1 *res, const Fr *expo) {
  pow256 tmp;
  pow256_from_Fr(tmp, expo);
  POINTonE1_mult_glv((POINTonE1 *)res, &BLS12_381_G1, tmp);
  vec_zero(&tmp, sizeof(tmp));
}

// Reads a scalar bytes and maps it to Fp using modular reduction.
// output is in Montgomery form.
// `in_len` must be less or equal to 96 bytes and must be a multiple of 8.
// This function is only used by `map_to_G1` where input is 64 bytes.
// input `in_len` is not checked to satisfy the conditions above.
static void map_96_bytes_to_Fp(Fp *a, const byte *in, int in_len) {
  vec768 tmp;
  vec_zero(&tmp, sizeof(tmp));
  limbs_from_be_bytes((limb_t *)tmp, in, in_len);
  redc_mont_384((limb_t *)a, tmp, BLS12_381_P, p0); // aR^(-2)
  Fp_mul_montg(a, a, (Fp *)BLS12_381_RRRR);         // aR
}

// maps bytes input `hash` to G1.
// `hash` must be `MAP_TO_G1_INPUT_LEN` (128 bytes)
// It uses construction 2 from section 5 in https://eprint.iacr.org/2019/403.pdf
int map_to_G1(E1 *h, const byte *hash, const int hash_len) {
  // sanity check of length
  if (hash_len != MAP_TO_G1_INPUT_LEN) {
    return INVALID;
  }
  // map to field elements
  Fp u[2];
  const int half = MAP_TO_G1_INPUT_LEN / 2;
  map_96_bytes_to_Fp(&u[0], hash, half);
  map_96_bytes_to_Fp(&u[1], hash + half, half);
  // map field elements to G1
  // inputs must be in Montgomery form
  map_to_g1((POINTonE1 *)h, (limb_t *)&u[0], (limb_t *)&u[1]);
  return VALID;
}

// maps the bytes to a point in G1.
// `len` should be at least Fr_BYTES.
// this is a testing file only, should not be used in any protocol!
void unsafe_map_bytes_to_G1(E1 *p, const byte *bytes, int len) {
  assert(len >= Fr_BYTES);
  // map to Fr
  Fr log;
  map_bytes_to_Fr(&log, bytes, len);
  // multiplies G1 generator by a random scalar
  G1_mult_gen(p, &log);
}

// maps bytes to a point in E1\G1.
// `len` must be at least 96 bytes.
// this is a testing function only, should not be used in any protocol!
void unsafe_map_bytes_to_G1complement(E1 *p, const byte *in, int in_len) {
  assert(in_len >= 96);
  Fp u;
  map_96_bytes_to_Fp(&u, in, 96);
  // map to E1's isogenous and then to E1
  map_to_isogenous_E1((POINTonE1 *)p, u);
  isogeny_map_to_E1((POINTonE1 *)p, (POINTonE1 *)p);
  // clear G1 order
  E1_mult(p, p, (Fr *)&BLS12_381_r);
}

// ------------------- E2 utilities

const E2 *BLS12_381_g2 = (const E2 *)&BLS12_381_G2;
const E2 *BLS12_381_minus_g2 = (const E2 *)&BLS12_381_NEG_G2;

// E2_read_bytes imports a E2(Fp^2) point from a buffer in a compressed or
// uncompressed form. The resulting point is guaranteed to be on curve E2 (no G2
// check is included).
// E2 point is in affine coordinates. This avoids further conversions
// when the point is used in multiple pairing computation.
//
// returns:
//    - BAD_ENCODING if the length is invalid or serialization header bits are
//    invalid
//    - BAD_VALUE if Fp^2 coordinates couldn't deserialize
//    - POINT_NOT_ON_CURVE if deserialized point isn't on E2
//    - VALID if deserialization is valid
//
// Note: can use with POINTonE2_Deserialize_BE and POINTonE2_Uncompress_Z,
//       and update the logic around G2 subgroup check.
ERROR E2_read_bytes(E2 *a, const byte *in, const int in_len) {
  // check the length
  if (in_len != G2_SER_BYTES) {
    return BAD_ENCODING;
  }

  // check the compression bit
  int compressed = in[0] >> 7;
  if ((compressed == 1) != (G2_SERIALIZATION == COMPRESSED)) {
    return BAD_ENCODING;
  }

  // check if the point in infinity
  int is_infinity = in[0] & 0x40;
  if (is_infinity) {
    // the remaining bits need to be cleared
    if (in[0] & 0x3F) {
      return BAD_ENCODING;
    }
    for (int i = 1; i < G2_SER_BYTES - 1; i++) {
      if (in[i]) {
        return BAD_ENCODING;
      }
    }
    E2_set_infty(a);
    return VALID;
  }

  // read the sign bit and check for consistency
  int y_sign = (in[0] >> 5) & 1;
  if (y_sign && (!compressed)) {
    return BAD_ENCODING;
  }

  // use a temporary buffer to mask the header bits and read a.x
  byte temp[Fp2_BYTES];
  memcpy(temp, in, Fp2_BYTES);
  temp[0] &= 0x1F; // clear the header bits
  ERROR ret = Fp2_read_bytes(&a->x, temp, sizeof(temp));
  if (ret != VALID) {
    return ret;
  }
  Fp2 *a_x = &(a->x);
  Fp_to_montg(&real(a_x), &real(a_x));
  Fp_to_montg(&imag(a_x), &imag(a_x));

  // set a.z to 1
  Fp2 *a_z = &(a->z);
  Fp_copy(&real(a_z), &BLS12_381_pR);
  Fp_set_zero(&imag(a_z));

  Fp2 *a_y = &(a->y);
  if (G2_SERIALIZATION == UNCOMPRESSED) {
    ret = Fp2_read_bytes(a_y, in + Fp2_BYTES, sizeof(a->y));
    if (ret != VALID) {
      return ret;
    }
    Fp_to_montg(&real(a_y), &real(a_y));
    Fp_to_montg(&imag(a_y), &imag(a_y));
    // check read point is on curve
    if (!E2_affine_on_curve(a)) {
      return POINT_NOT_ON_CURVE;
    }
    return VALID;
  }

  // compute the possible square root
  Fp2_squ_montg(a_y, a_x);
  Fp2_mul_montg(a_y, a_y, a_x);  // x^3
  Fp2_add(a_y, a_y, &B_E2);      // B_E2 is already in Montg form
  if (!Fp2_sqrt_montg(a_y, a_y)) // check whether x^3+b is a quadratic residue
    return POINT_NOT_ON_CURVE;

  // resulting (x,y) is guaranteed to be on curve (y is already in Montg form)
  if (Fp2_get_sign(a_y) != y_sign) {
    Fp2_neg(a_y, a_y); // flip y sign if needed
  }
  return VALID;
}

// E2_write_bytes exports a point in E2(Fp^2) to a buffer in a compressed or
// uncompressed form. It assumes buffer is of length G2_SER_BYTES The
// serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
void E2_write_bytes(byte *out, const E2 *a) {
  if (E2_is_infty(a)) {
    // set the infinity bit
    out[0] = (G2_SERIALIZATION << 7) | (1 << 6);
    memset(out + 1, 0, G2_SER_BYTES - 1);
    return;
  }
  E2 tmp;
  E2_to_affine(&tmp, a);

  Fp2 *t_x = &(tmp.x);
  Fp_from_montg(&real(t_x), &real(t_x));
  Fp_from_montg(&imag(t_x), &imag(t_x));
  Fp2_write_bytes(out, t_x);

  Fp2 *t_y = &(tmp.y);
  if (G2_SERIALIZATION == COMPRESSED) {
    out[0] |= (Fp2_get_sign(t_y) << 5);
  } else {
    Fp_from_montg(&real(t_y), &real(t_y));
    Fp_from_montg(&imag(t_y), &imag(t_y));
    Fp2_write_bytes(out + Fp2_BYTES, t_y);
  }

  out[0] |= (G2_SERIALIZATION << 7);
}

// set p to infinity
void E2_set_infty(E2 *p) {
  // BLST infinity points are defined by Z=0
  vec_zero(p->z, sizeof(p->z));
}

// check if `p` is infinity
bool E2_is_infty(const E2 *p) {
  // BLST infinity points are defined by Z=0
  return vec_is_zero(p->z, sizeof(p->z));
}

// checks affine point `p` is in E2
bool E2_affine_on_curve(const E2 *p) {
  // BLST's `POINTonE2_affine_on_curve` does not include the infinity case!
  return POINTonE2_affine_on_curve((POINTonE2_affine *)p) | E2_is_infty(p);
}

// checks p1 == p2
bool E2_is_equal(const E2 *p1, const E2 *p2) {
  // `POINTonE2_is_equal` includes the infinity case
  return POINTonE2_is_equal((const POINTonE2 *)p1, (const POINTonE2 *)p2);
}

// res = p
void E2_copy(E2 *res, const E2 *p) {
  if ((uptr_t)p == (uptr_t)res) {
    return;
  }
  vec_copy(res, p, sizeof(E2));
}

// converts an E2 point from Jacobian into affine coordinates (z=1)
void E2_to_affine(E2 *res, const E2 *p) {
  // optimize in case coordinates are already affine
  if (vec_is_equal(p->z, BLS12_381_Rx.p2, sizeof(p->z))) {
    E2_copy(res, p);
    return;
  }
  // convert from Jacobian
  POINTonE2_from_Jacobian((POINTonE2 *)res, (const POINTonE2 *)p);
}

// generic point addition that must handle doubling and points at infinity
void E2_add(E2 *res, const E2 *a, const E2 *b) {
  POINTonE2_dadd((POINTonE2 *)res, (POINTonE2 *)a, (POINTonE2 *)b, NULL);
}

// generic point double that must handle point at infinity
static void E2_double(E2 *res, const E2 *a) {
  POINTonE2_double((POINTonE2 *)res, (POINTonE2 *)a);
}

// Point negation: res = -a
void E2_neg(E2 *res, const E2 *a) {
  E2_copy(res, a);
  POINTonE2_cneg((POINTonE2 *)res, 1);
}

// Exponentiation of a generic point `a` in E2, res = expo.a
void E2_mult(E2 *res, const E2 *p, const Fr *expo) {
  pow256 tmp;
  pow256_from_Fr(tmp, expo);
  POINTonE2_mult_gls((POINTonE2 *)res, (POINTonE2 *)p, tmp);
  vec_zero(&tmp, sizeof(tmp));
}

// Exponentiation of a generic point `a` in E2 by a byte exponent,
// using a classic double-and-add algorithm (non constant-time)
void E2_mult_small_expo(E2 *res, const E2 *p, const byte expo) {
  // return early if expo is zero
  if (expo == 0) {
    E2_set_infty(res);
    return;
  }
  // expo is non zero

  byte mask = 1 << 7;
  // process the most significant zero bits
  while ((expo & mask) == 0) {
    mask >>= 1;
  }

  // process the first `1` bit
  E2 tmp;
  E2_copy(&tmp, p);
  mask >>= 1;
  // scan the remaining bits
  for (; mask != 0; mask >>= 1) {
    E2_double(&tmp, &tmp);
    if (expo & mask) {
      E2_add(&tmp, &tmp, p);
    }
  }
  E2_copy(res, &tmp);
}

// Exponentiation of generator g2 of G2, res = expo.g2
void G2_mult_gen(E2 *res, const Fr *expo) {
  pow256 tmp;
  pow256_from_Fr(tmp, expo);
  POINTonE2_mult_gls((POINTonE2 *)res, (POINTonE2 *)BLS12_381_g2, tmp);
  vec_zero(&tmp, sizeof(tmp));
}

// Exponentiation of generator g2 of G2, res = expo.g2.
//
// Result is converted to affine. This is useful for results being used multiple
// times in pairings. Conversion to affine saves later pre-pairing conversions.
void G2_mult_gen_to_affine(E2 *res, const Fr *expo) {
  G2_mult_gen(res, expo);
  E2_to_affine(res, res);
}

// checks if input E2 point is on the subgroup G2.
// It assumes input `p` is on E2.
bool E2_in_G2(const E2 *p) {
  // currently uses Scott method
  return POINTonE2_in_G2((const POINTonE2 *)p);
}

// computes the sum of the E2 array elements `y[i]` and writes it in `sum`
void E2_sum_vector(E2 *sum, const E2 *y, const int y_len) {
  E2_set_infty(sum);
  for (int i = 0; i < y_len; i++) {
    E2_add(sum, sum, &y[i]);
  }
}

// computes the sum of the E2 array elements `y[i]`, converts it
// to affine coordinates, and writes it in `sum`.
//
// Result is converted to affine. This is useful for results being used multiple
// times in pairings. Conversion to affine saves later pre-pairing conversions.
void E2_sum_vector_to_affine(E2 *sum, const E2 *y, const int y_len) {
  E2_sum_vector(sum, y, y_len);
  E2_to_affine(sum, sum);
}

// Subtracts all G2 array elements `y` from an element `x` and writes the
// result in res.
void E2_subtract_vector(E2 *res, const E2 *x, const E2 *y, const int y_len) {
  E2_sum_vector(res, y, y_len);
  E2_neg(res, res);
  E2_add(res, x, res);
}

// maps the bytes to a point in G2.
// `in_len` should be at least Fr_BYTES.
// this is a testing tool only, it should not be used in any protocol!
void unsafe_map_bytes_to_G2(E2 *p, const byte *in, int in_len) {
  assert(in_len >= Fr_BYTES);
  // map to Fr
  Fr log;
  map_bytes_to_Fr(&log, in, in_len);
  // multiplies G2 generator by a random scalar
  G2_mult_gen(p, &log);
}

// maps `in` to a point in E2\G2 and stores it in p.
// `len` should be at least 192.
// this is a testing tool only, it should not be used in any protocol!
void unsafe_map_bytes_to_G2complement(E2 *p, const byte *in, int in_len) {
  assert(in_len >= 192);
  Fp2 u;
  map_96_bytes_to_Fp(&real(&u), in, 96);
  map_96_bytes_to_Fp(&imag(&u), in + 96, 96);
  // map to E2's isogenous and then to E2
  map_to_isogenous_E2((POINTonE2 *)p, u);
  isogeny_map_to_E2((POINTonE2 *)p, (POINTonE2 *)p);
  // clear G2 order
  E2_mult(p, p, (Fr *)&BLS12_381_r);
}

// ------------------- Pairing utilities

bool Fp12_is_one(Fp12 *a) {
  return vec_is_equal(a, BLS12_381_Rx.p12, sizeof(Fp12));
}

void Fp12_set_one(Fp12 *a) { vec_copy(a, BLS12_381_Rx.p12, sizeof(Fp12)); }

// computes e(p[0], q[0]) * ... * e(q[len-1], q[len-1])
// by optimizing a common final exponentiation for all pairings.
// result is stored in `res`.
// It assumes `p` and `q` are correctly initialized and all
// p[i] and q[i] are respectively on G1 and G2 (it does not
// check their memberships).
void Fp12_multi_pairing(Fp12 *res, const E1 *p, const E2 *q, const int len) {
  // easier access pointer
  vec384fp6 *res_vec = (vec384fp6 *)res;
  // N_MAX is defined within BLST. It should represent a good tradeoff of the
  // max number of miller loops to be batched in one call to `miller_loop_n`.
  // miller_loop_n expects an array of `POINTonEx_affine`.
  POINTonE1_affine p_aff[N_MAX];
  POINTonE2_affine q_aff[N_MAX];
  int n = 0; // the number of couples (p,q) held in p_aff and q_aff
  int init_flag = 0;

  for (int i = 0; i < len; i++) {
    if (E1_is_infty(p + i) || E2_is_infty(q + i)) {
      continue;
    }
    // `miller_loop_n` expects affine coordinates in a `POINTonEx_affine` array.
    // `POINTonEx_affine` has a different size than `POINTonEx` and `Ex` !
    E1 tmp1;
    E1_to_affine(&tmp1, p + i);
    vec_copy(p_aff + n, &tmp1, sizeof(POINTonE1_affine));
    E2 tmp2;
    E2_to_affine(&tmp2, q + i);
    vec_copy(q_aff + n, &tmp2, sizeof(POINTonE2_affine));
    n++;
    // if p_aff and q_aff are filled, batch `N_MAX` miller loops
    if (n == N_MAX) {
      if (!init_flag) {
        miller_loop_n(res_vec, q_aff, p_aff, N_MAX);
        init_flag = 1;
      } else {
        vec384fp12 tmp;
        miller_loop_n(tmp, q_aff, p_aff, N_MAX);
        mul_fp12(res_vec, res_vec, tmp);
      }
      n = 0;
    }
  }
  // if p_aff and q_aff aren't empty,
  // the remaining couples are also batched in `n` miller loops
  if (n > 0) {
    if (!init_flag) {
      miller_loop_n(res_vec, q_aff, p_aff, n);
      init_flag = 1;
    } else {
      vec384fp12 tmp;
      miller_loop_n(tmp, q_aff, p_aff, n);
      mul_fp12(res_vec, res_vec, tmp);
    }
  }

  // check if no miller loop was computed
  if (!init_flag) {
    Fp12_set_one(res);
  }
  final_exp(res_vec, res_vec);
}

// ------------------- Other utilities

// This is a testing function and is not used in exported functions
// It uses an expand message XMD based on SHA2-256.
void xmd_sha256(byte *hash, int len_hash, byte *msg, int len_msg, byte *dst,
                int len_dst) {
  expand_message_xmd(hash, len_hash, NULL, 0, msg, len_msg, dst, len_dst);
}

// DEBUG printing functions
#ifdef DEBUG
void bytes_print_(char *s, byte *data, int len) {
  if (strlen(s))
    printf("[%s]:\n", s);
  for (int i = 0; i < len; i++)
    printf("%02X,", data[i]);
  printf("\n");
}

void Fr_print_(char *s, Fr *a) {
  if (strlen(s))
    printf("[%s]:\n", s);
  limb_t *p = (limb_t *)(a) + Fr_LIMBS;
  for (int i = 0; i < Fr_LIMBS; i++)
    printf("%016llX", *(--p));
  printf("\n");
}

void Fp_print_(char *s, const Fp *a) {
  if (strlen(s))
    printf("[%s]:\n", s);
  Fp tmp;
  Fp_from_montg(&tmp, a);
  limb_t *p = (limb_t *)(&tmp) + Fp_LIMBS;
  for (int i = 0; i < Fp_LIMBS; i++)
    printf("%016llX ", *(--p));
  printf("\n");
}

void Fp2_print_(char *s, const Fp2 *a) {
  if (strlen(s))
    printf("[%s]:\n", s);
  Fp_print_("", &real(a));
  Fp_print_("", &imag(a));
}

void Fp12_print_(char *s, const Fp12 *a) {
  if (strlen(s))
    printf("[%s]:\n", s);
  for (int i = 0; i < 2; i++) {
    vec384fp6 *a_ = (vec384fp6 *)a + i;
    for (int j = 0; j < 3; j++) {
      vec384fp2 *a__ = (vec384fp2 *)a_ + j;
      Fp2_print_("", a__);
    }
  }
}

void E1_print_(char *s, const E1 *p, const int jacob) {
  E1 a;
  E1_copy(&a, p);
  if (!jacob)
    E1_to_affine(&a, &a);
  if (strlen(s))
    printf("[%s]:\n", s);
  Fp_print_(".x", &(a.x));
  Fp_print_(".y", &(a.y));
  if (jacob)
    Fp_print_(".z", &(a.z));
}

void E2_print_(char *s, const E2 *p, const int jacob) {
  E2 a;
  E2_copy(&a, p);
  if (!jacob)
    E2_to_affine(&a, &a);
  if (strlen(s))
    printf("[%s]:\n", s);
  Fp2_print_("", &(a.x));
  Fp2_print_("", &(a.y));
  if (jacob)
    Fp2_print_("", &(a.z));
}

#endif
