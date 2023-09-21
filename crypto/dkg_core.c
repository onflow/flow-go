#include "dkg_include.h"

// computes P(x) = a_0 + a_1*x + .. + a_n x^n in F_r
// where `x` is a small integer (byte) and `degree` is P's degree n.
// P(x) is written in `out` and P(x).g2 is written in `y` if `y` is non NULL.
void Fr_polynomial_image_write(byte *out, E2 *y, const Fr *a, const int degree,
                               const byte x) {
  Fr image;
  Fr_polynomial_image(&image, y, a, degree, x);
  // exports the result
  Fr_write_bytes(out, &image);
}

// computes P(x) = a_0 + a_1 * x + .. + a_n * x^n  where P is in Fr[X].
// a_i are all in Fr, `degree` is P's degree, x is a small integer less than
// `MAX_IND` (currently 255).
// The function writes P(x) in `image` and P(x).g2 in `y` if `y` is non NULL.
void Fr_polynomial_image(Fr *image, E2 *y, const Fr *a, const int degree,
                         const byte x) {
  Fr_set_zero(image);
  // convert `x` to Montgomery form
  Fr xR;
  Fr_set_limb(&xR, (limb_t)x);
  Fr_to_montg(&xR, &xR);

  for (int i = degree; i >= 0; i--) {
    Fr_mul_montg(image, image, &xR);
    Fr_add(image, image, &a[i]); // image is in normal form
  }
  // compute y = P(x).g2
  if (y) {
    G2_mult_gen(y, image);
  }
}

// computes Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
// and stores the point in y.
//  - A_i being G2 points
//  - x being a small scalar (less than `MAX_IND`)
static void E2_polynomial_image(E2 *y, const E2 *A, const int degree,
                                const byte x) {
  E2_set_infty(y);
  for (int i = degree; i >= 0; i--) {
    E2_mult_small_expo(y, y, x);
    E2_add(y, y, &A[i]);
  }
}

// computes y[i] = Q(i+1) for all participants i ( 0 <= i < len_y)
// where Q(x) = A_0 + A_1*x + ... +  A_n*x^n
//  - A_i being G2 points
//  - x being a small scalar (less than `MAX_IND`)
void E2_polynomial_images(E2 *y, const int len_y, const E2 *A,
                          const int degree) {
  for (byte i = 0; i < len_y; i++) {
    // y[i] = Q(i+1)
    E2_polynomial_image(y + i, A, degree, i + 1);
  }
}

// export an array of E2 into an array of bytes by concatenating
// all serializations of E2 points in order.
// the array must be of length (A_len * G2_SER_BYTES).
void E2_vector_write_bytes(byte *out, const E2 *A, const int A_len) {
  byte *p = out;
  for (int i = 0; i < A_len; i++) {
    E2_write_bytes(p, &A[i]);
    p += G2_SER_BYTES;
  }
}

// The function imports an array of `A_len` E2 points from a concatenated array
// of bytes. The bytes array is supposed to be of size (A_len * G2_SER_BYTES).
//
// If return is `VALID`, output vector is guaranteed to be in G2.
// It returns other errors if at least one input isn't a serialization of a E2
// point, or an input E2 point isn't in G2.
// returns:
//    - BAD_ENCODING if the serialization header bits of at least one input are
//    invalid.
//    - BAD_VALUE if Fp^2 coordinates of at least one input couldn't
//    deserialize.
//    - POINT_NOT_ON_CURVE if  at least one input deserialized point isn't on
//    E2.
//    - POINT_NOT_IN_GROUP if at least one E2 point isn't in G2.
//    - VALID if deserialization of all points to G2 is valid.
ERROR G2_vector_read_bytes(E2 *A, const byte *src, const int A_len) {
  byte *p = (byte *)src;
  for (int i = 0; i < A_len; i++) {
    int read_ret = E2_read_bytes(&A[i], p, G2_SER_BYTES);
    if (read_ret != VALID) {
      return read_ret;
    }
    if (!E2_in_G2(&A[i])) {
      return POINT_NOT_IN_GROUP;
    }
    p += G2_SER_BYTES;
  }
  return VALID;
}

// checks the discrete log relationship in G2.
// - returns 1 if g2^x = y, where g2 is the generator of G2
// - returns 0 otherwise.
bool G2_check_log(const Fr *x, const E2 *y) {
  E2 tmp;
  G2_mult_gen(&tmp, x);
  return E2_is_equal(&tmp, y);
}
