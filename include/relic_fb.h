/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @defgroup fb Binary field arithmetic.
 */

/**
 * @file
 *
 * Interface of the binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup fb
 */

#ifndef RELIC_FB_H
#define RELIC_FB_H

#include "relic_dv.h"
#include "relic_conf.h"
#include "relic_types.h"

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Precision in bits of a binary field element.
 */
#define FB_BITS 	((int)FB_POLYN)

/**
 * Size in bits of a digit.
 */
#define FB_DIGIT	((int)DIGIT)

/**
 * Logarithm of the digit size in base 2.
 */
#define FB_DIG_LOG	((int)DIGIT_LOG)

/**
 * Size in digits of a block sufficient to store a binary field element.
 */
#define FB_DIGS		((int)((FB_BITS)/(FB_DIGIT) + (FB_BITS % FB_DIGIT > 0)))

/**
 * Size in bytes of a block sufficient to store a binary field element.
 */
#define FB_BYTES 	(FB_DIGS * sizeof(dig_t))

/**
 * Finite field identifiers.
 */
enum {
	/** Toy pentanomial. */
	PENTA_64 = 1,
	/** Hankerson's trinomial for GLS curves. */
	TRINO_113,
	/** Hankerson's trinomial for GLS curves. */
	TRINO_127,
	/** Pentanomial for ECC2K-130 challenge. */
	PENTA_131,
	/** NIST 163-bit fast reduction polynomial. */
	NIST_163,
	/** Square-root friendly 163-bit polynomial. */
	SQRT_163,
	/** Example with 193 bits for Itoh-Tsuji. */
	TRINO_193,
	/** NIST 233-bit fast reduction polynomial. */
	NIST_233,
	/** Square-root friendly 233-bit polynomial. */
	SQRT_233,
	/** SECG 239-bit fast reduction polynomial. */
	SECG_239,
	/** Square-root friendly 239-bit polynomial. */
	SQRT_239,
	/** Square-root friendly 251-bit polynomial. */
	SQRT_251,
	/** eBATS curve_2_251 pentanomial. */
	PENTA_251,
	/** Hankerson's trinomial for halving curve. */
	TRINO_257,
	/** Scott's 271-bit pairing-friendly trinomial. */
	TRINO_271,
	/** Scott's 271-bit pairing-friendly pentanomial. */
	PENTA_271,
	/** NIST 283-bit fast reduction polynomial. */
	NIST_283,
	/** Square-root friendly 283-bit polynomial. */
	SQRT_283,
	/** Scott's 271-bit pairing-friendly trinomial. */
	TRINO_353,
	/** Detrey's trinomial for genus 2 curves. */
	TRINO_367,
	/** NIST 409-bit fast reduction polynomial. */
	NIST_409,
	/** Hankerson's trinomial for genus 2 curves. */
	TRINO_439,
	/** NIST 571-bit fast reduction polynomial. */
	NIST_571,
	/** Square-root friendly 571-bit polynomial. */
	SQRT_571,
	/** Scott's 1223-bit pairing-friendly trinomial. */
	TRINO_1223
};

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a binary field element.
 */
#if ALLOC == AUTO
typedef align dig_t fb_t[FB_DIGS + PADDING(FB_BYTES)/sizeof(dig_t)];
#else
typedef dig_t *fb_t;
#endif

/**
 * Represents a binary field element with automatic memory allocation.
 */
typedef align dig_t fb_st[FB_DIGS + PADDING(FB_BYTES)/sizeof(dig_t)];

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a binary field element with a null value.
 *
 * @param[out] A			- the binary field element to initialize.
 */
#if ALLOC == AUTO
#define fb_null(A)			/* empty */
#else
#define fb_null(A)			A = NULL;
#endif

/**
 * Calls a function to allocate a binary field element.
 *
 * @param[out] A			- the new binary field element.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define fb_new(A)			dv_new_dynam((dv_t *)&(A), FB_DIGS)
#elif ALLOC == STATIC
#define fb_new(A)			dv_new_statc((dv_t *)&(A), FB_DIGS)
#elif ALLOC == AUTO
#define fb_new(A)			/* empty */
#elif ALLOC == STACK
#define fb_new(A)															\
	A = (dig_t *)alloca(FB_BYTES + PADDING(FB_BYTES));						\
	A = (dig_t *)ALIGNED(A);												\

#endif

/**
 * Calls a function to free a binary field element.
 *
 * @param[out] A			- the binary field element to clean and free.
 */
#if ALLOC == DYNAMIC
#define fb_free(A)			dv_free_dynam((dv_t *)&(A))
#elif ALLOC == STATIC
#define fb_free(A)			dv_free_statc((dv_t *)&(A))
#elif ALLOC == AUTO
#define fb_free(A)			/* empty */
#elif ALLOC == STACK
#define fb_free(A)			A = NULL;
#endif

/**
 * Multiples two binary field elements. Computes c = a * b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first binary field element to multiply.
 * @param[in] B				- the second binary field element to multiply.
 */
#if FB_KARAT > 0
#define fb_mul(C, A, B)	fb_mul_karat(C, A, B)
#elif FB_MUL == BASIC
#define fb_mul(C, A, B)	fb_mul_basic(C, A, B)
#elif FB_MUL == INTEG
#define fb_mul(C, A, B)	fb_mul_integ(C, A, B)
#elif FB_MUL == LCOMB
#define fb_mul(C, A, B)	fb_mul_lcomb(C, A, B)
#elif FB_MUL == RCOMB
#define fb_mul(C, A, B)	fb_mul_rcomb(C, A, B)
#elif FB_MUL == LODAH
#define fb_mul(C, A, B)	fb_mul_lodah(C, A, B)
#endif

/**
 * Squares a binary field element. Computes c = a * a.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the binary field element to square.
 */
#if FB_SQR == BASIC
#define fb_sqr(C, A)	fb_sqr_basic(C, A)
#elif FB_SQR == TABLE
#define fb_sqr(C, A)	fb_sqr_table(C, A)
#elif FB_SQR == INTEG
#define fb_sqr(C, A)	fb_sqr_integ(C, A)
#endif

/**
 * Extracts the square root of a binary field element. Computes c = a^(1/2).
 *
 * @param[out] C			- the result.
 * @param[in] A				- the binary field element.
 */
#if FB_SRT == BASIC
#define fb_srt(C, A)	fb_srt_basic(C, A)
#elif FB_SRT == QUICK
#define fb_srt(C, A)	fb_srt_quick(C, A)
#endif

/**
 * Reduces a multiplication result modulo a binary irreducible polynomial.
 * Computes c = a mod f(z).
 *
 * @param[out] C			- the result.
 * @param[in] A				- the multiplication result to reduce.
 */
#if FB_RDC == BASIC
#define fb_rdc(C, A)	fb_rdc_basic(C, A)
#elif FB_RDC == QUICK
#define fb_rdc(C, A)	fb_rdc_quick(C, A)
#endif

/**
 * Compute the trace of a binary field element. Computes c = Tr(a).
 *
 * @param[in] A				- the binary field element.
 * @return the trace of the binary field element.
 */
#if FB_TRC == BASIC
#define fb_trc(A)		fb_trc_basic(A)
#elif FB_TRC == QUICK
#define fb_trc(A)		fb_trc_quick(A)
#endif

/**
 * Solves a quadratic equation for c, Tr(a) = 0. Computes c such that
 * c^2 + c = a.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the binary field element.
 */
#if FB_SLV == BASIC
#define fb_slv(C, A)	fb_slv_basic(C, A)
#elif FB_SLV == QUICK
#define fb_slv(C, A)	fb_slv_quick(C, A)
#endif

/**
 * Inverts a binary field element. Computes c = a^{-1}.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the binary field element to invert.
 */
#if FB_INV == BASIC
#define fb_inv(C, A)	fb_inv_basic(C, A)
#elif FB_INV == BINAR
#define fb_inv(C, A)	fb_inv_binar(C, A)
#elif FB_INV == LOWER
#define fb_inv(C, A)	fb_inv_lower(C, A)
#elif FB_INV == EXGCD
#define fb_inv(C, A)	fb_inv_exgcd(C, A)
#elif FB_INV == ALMOS
#define fb_inv(C, A)	fb_inv_almos(C, A)
#elif FB_INV == ITOHT
#define fb_inv(C, A)	fb_inv_itoht(C, A)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the binary field arithmetic layer.
 */
void fb_poly_init(void);

/**
 * Finalizes the binary field arithmetic layer.
 */
void fb_poly_clean(void);

/**
 * Returns the irreducible polynomial f(z) configured for the binary field.
 *
 * @return the irreducible polynomial.
 */
dig_t *fb_poly_get(void);

/**
 * Configures the irreducible polynomial of the binary field as a dense
 * polynomial.
 *
 * @param[in] f				- the new irreducible polynomial.
 */
void fb_poly_set_dense(fb_t f);

/**
 * Configures a trinomial as the irreducible polynomial by its non-zero
 * coefficients. The other coefficients are FB_BITS and 0.
 *
 * @param[in] a				- the second coefficient.
 */
void fb_poly_set_trino(int a);

/**
 * Configures a pentanomial as the binary field modulo by its non-zero
 * coefficients. The other coefficients are FB_BITS and 0.
 *
 * @param[in] a				- the second coefficient.
 * @param[in] b				- the third coefficient.
 * @param[in] c				- the fourth coefficient.
 */
void fb_poly_set_penta(int a, int b, int c);

/**
 * Returns the square root of z.
 *
 * @return the square root of z.
 */
dig_t *fb_poly_get_srz(void);

/**
 * Returns sqrt(z) * (i represented as a polynomial).
 *
 * @return the precomputed result.
 */
dig_t *fb_poly_tab_srz(int i);

/**
 * Returns a table for accelerating repeated squarings.
 *
 * @param the number of the table.
 * @return the precomputed result.
 */
dig_t *fb_poly_tab_sqr(int i);

/**
 * Returns an addition chain for (FB_BITS - 1).
 *
 * @param[out] len		- the number of elements in the addition chain.
 *
 * @return a pointer to the addition chain.
 */
int *fb_poly_get_chain(int *len);

/**
 * Returns the non-zero coefficients of the configured trinomial or pentanomial.
 * If b is -1, the irreducible polynomial configured is a trinomial.
 * The other coefficients are FB_BITS and 0.
 *
 * @param[out] a			- the second coefficient.
 * @param[out] b			- the third coefficient.
 * @param[out] c			- the fourth coefficient.
 */
void fb_poly_get_rdc(int *a, int *b, int *c);

/**
 * Returns the non-zero bits used to compute the trace function. The -1
 * coefficient is the last coefficient.
 *
 * @param[out] a			- the first coefficient.
 * @param[out] b			- the second coefficient.
 * @param[out] c			- the third coefficient.
 */
void fb_poly_get_trc(int *a, int *b, int *c);

/**
 * Returns the table of precomputed half-traces.
 *
 * @return the table of half-traces.
 */
dig_t *fb_poly_get_slv(void);

/**
 * Assigns a standard irreducible polynomial as modulo of the binary field.
 *
 * @param[in] param			- the standardized polynomial identifier.
 */
void fb_param_set(int param);

/**
 * Configures some finite field parameters for the current security level.
 */
void fb_param_set_any(void);

/**
 * Prints the currently configured irreducible polynomial.
 */
void fb_param_print(void);

/**
 * Adds a binary field element and the irreducible polynomial. Computes
 * c = a + f(z).
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the binary field element.
 */
void fb_poly_add(fb_t c, fb_t a);

/**
 * Subtracts the irreducible polynomial from a binary field element. Computes
 * c = a - f(z).
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the binary field element.
 */
void fb_poly_sub(fb_t c, fb_t a);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to copy.
 */
void fb_copy(fb_t c, fb_t a);

/**
 * Negates a binary field element.
 *
 * @param[out] c			- the result.
 * @param[out] a			- the binary field element to negate.
 */
void fb_neg(fb_t c, fb_t a);

/**
 * Assigns zero to a binary field element.
 *
 * @param[out] a			- the binary field element to assign.
 */
void fb_zero(fb_t a);

/**
 * Tests if a binary field element is zero or not.
 *
 * @param[in] a				- the binary field element to test.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int fb_is_zero(fb_t a);

/**
 * Tests if the bit in a given position is non-zero on a binary field element.
 *
 * @param[in] a				- the binary field element to test.
 * @param[in] bit			- the bit position.
 * @return 0 is the bit is zero, not zero otherwise.
 */
int fb_test_bit(fb_t a, int bit);

/**
 * Reads the bit stored in the given position on a binary field element.
 *
 * @param[in] a				- the binary field element.
 * @param[in] bit			- the bit position.
 * @return the bit value.
 */
int fb_get_bit(fb_t a, int bit);

/**
 * Stores a bit in a given position on a binary field element.
 *
 * @param[out] a			- the binary field element.
 * @param[in] bit			- the bit position.
 * @param[in] value			- the bit value.
 */
void fb_set_bit(fb_t a, int bit, int value);

/**
 * Assigns a small positive polynomial to a binary field element.
 *
 * The degree of the polynomial must be smaller than FB_DIGIT.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the small polynomial to assign.
 */
void fb_set_dig(fb_t c, dig_t a);

/**
 * Returns the number of bits of a binary field element.
 *
 * @param[in] a				- the binary field element.
 * @return the number of bits.
 */
int fb_bits(fb_t a);

/**
 * Assigns a random value to a binary field element.
 *
 * @param[out] a			- the binary field element to assign.
 */
void fb_rand(fb_t a);

/**
 * Prints a binary field element to standard output.
 *
 * @param[in] a				- the binary field element to print.
 */
void fb_print(fb_t a);

/**
 * Returns the number of digits in radix necessary to store a binary field
 * element. The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] size			- the result.
 * @param[in] a				- the binary field element.
 * @param[in] radix			- the radix.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fb_size(int *size, fb_t a, int radix);

/**
 * Reads a binary field element from a string in a given radix. The radix must
 * be a power of 2 included in the interval [2, 64].
 *
 * @param[out] a			- the result.
 * @param[in] str			- the string.
 * @param[in] len			- the size of the string.
 * @param[in] radix			- the radix.
 * @throw ERR_NO_BUFFER		- if the string if too long.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fb_read(fb_t a, const char *str, int len, int radix);

/**
 * Writes a binary field element to a string in a given radix. The radix must
 * be a power of 2 included in the interval [2, 64].
 *
 * @param[out] str			- the string.
 * @param[in] len			- the buffer capacity.
 * @param[in] a				- the binary field element to write.
 * @param[in] radix			- the radix.
 * @throw ERR_NO_BUFFER		- if the buffer capacity is insufficient.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fb_write(char *str, int len, fb_t a, int radix);

/**
 * Returns the result of a comparison between two binary field elements.
 *
 * @param[in] a				- the first binary field element.
 * @param[in] b				- the second binary field element.
 * @return FB_LT if a < b, FB_EQ if a == b and FB_GT if a > b.
 */
int fb_cmp(fb_t a, fb_t b);

/**
 * Returns the result of a comparison between a binary field element
 * and a small binary field element.
 *
 * @param[in] a				- the binary field element.
 * @param[in] b				- the small binary field element.
 * @return FB_LT if a < b, FB_EQ if a == b and FB_GT if a > b.
 */
int fb_cmp_dig(fb_t a, dig_t b);

/**
 * Adds two binary field elements. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to add.
 * @param[in] b				- the second binary field element to add.
 */
void fb_add(fb_t c, fb_t a, fb_t b);

/**
 * Adds a binary field element and a small binary field element.
 * Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to add.
 * @param[in] b				- the small binary field element to add.
 */
void fb_add_dig(fb_t c, fb_t a, dig_t b);

/**
 * Subtracts a binary field element from another. Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element.
 * @param[in] b				- the binary field element to subtract.
 */
void fb_sub(fb_t c, fb_t a, fb_t b);

/**
 * Subtracts a small binary field element from a binary field element.
 * Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element.
 * @param[in] b				- the small binary field element to subtract.
 */
void fb_sub_dig(fb_t c, fb_t a, dig_t b);

/**
 * Multiples two binary field elements using Shift-and-add multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to multiply.
 * @param[in] b				- the second binary field element to multiply.
 */
void fb_mul_basic(fb_t c, fb_t a, fb_t b);

/**
 * Multiples two binary field elements using multiplication integrated with
 * modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to multiply.
 * @param[in] b				- the second binary field element to multiply.
 */
void fb_mul_integ(fb_t c, fb_t a, fb_t b);

/**
 * Multiples two binary field elements using Left-to-right comb multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to multiply.
 * @param[in] b				- the second binary field element to multiply.
 */
void fb_mul_lcomb(fb_t c, fb_t a, fb_t b);

/**
 * Multiples two binary field elements using Right-to-left comb multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to multiply.
 * @param[in] b				- the second binary field element to multiply.
 */
void fb_mul_rcomb(fb_t c, fb_t a, fb_t b);

/**
 * Multiples two binary field elements using López-Dahab multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element to multiply.
 * @param[in] b				- the second binary field element to multiply.
 */
void fb_mul_lodah(fb_t c, fb_t a, fb_t b);

/**
 * Multiplies a binary field element by a small binary field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element.
 * @param[in] b				- the small binary field element to multiply.
 */
void fb_mul_dig(fb_t c, fb_t a, dig_t b);

/**
 * Multiples two binary field elements using Karatsuba multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element.
 * @param[in] b				- the second binary field element.
 */
void fb_mul_karat(fb_t c, fb_t a, fb_t b);

/**
 * Squares a binary field element using bit-manipulation squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to square.
 */
void fb_sqr_basic(fb_t c, fb_t a);

/**
 * Squares a binary field element with integrated modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to square.
 */
void fb_sqr_integ(fb_t c, fb_t a);

/**
 * Squares a binary field element using table-based squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to square.
 */
void fb_sqr_table(fb_t c, fb_t a);

/**
 * Shifts a binary field element to the left. Computes c = a * z^bits mod f(z).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to shift.
 * @param[in] bits			- the number of bits to shift.
 */
void fb_lsh(fb_t c, fb_t a, int bits);

/**
* Shifts a binary field element to the right. Computes c = a / (z^bits).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to shift.
 * @param[in] bits			- the number of bits to shift.
 */
void fb_rsh(fb_t c, fb_t a, int bits);

/**
 * Exponentiates a binary field element. Computes c = a^b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the basis.
 * @param[in] b				- the exponent.
 */
void fb_exp(fb_t c, fb_t a, fb_t b);

/**
 * Reduces a multiplication result modulo an irreducible polynomial using
 * shift-and-add modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fb_rdc_basic(fb_t c, dv_t a);

/**
 * Reduces a multiplication result modulo a trinomial or pentanomial.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fb_rdc_quick(fb_t c, dv_t a);

/**
 * Extracts the square root of a binary field element using repeated squaring.
 * Computes c = a^{1/2}.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to take a square root.
 */
void fb_srt_basic(fb_t c, fb_t a);

/**
 * Extracts the square root of a binary field element using a fast square root
 * extraction algorithm.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to take a square root.
 */
void fb_srt_quick(fb_t c, fb_t a);

/**
 * Computes the trace of a binary field element using repeated squaring.
 * Returns Tr(a).
 *
 * @param[in] a				- the binary field element.
 * @return the trace of the binary field element.
 */
dig_t fb_trc_basic(fb_t a);

/**
 * Computes the trace of a binary field element using a fast trace computation
 * algorithm. Returns Tr(a).
 *
 * @param[in] a				- the binary field element.
 * @return the trace of the binary field element.
 */
dig_t fb_trc_quick(fb_t a);

/**
 * Inverts a binary field element using Fermat's Little Theorem.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_basic(fb_t c, fb_t a);

/**
 * Inverts a binary field element using the binary method.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_binar(fb_t c, fb_t a);

/**
 * Inverts a binary field element using the Extended Euclidean algorithm.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_exgcd(fb_t c, fb_t a);

/**
 * Inverts a binary field element using the Almost Inverse algorithm.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_almos(fb_t c, fb_t a);

/**
 * Inverts a binary field element using Itoh-Tsuji inversion.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_itoht(fb_t c, fb_t a);

/**
 * Inverts a binary field element using a direct call to the lower layer.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to invert.
 */
void fb_inv_lower(fb_t c, fb_t a);

/**
 * Inverts multiple binary field elements.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field elements to invert.
 * @param[in] n				- the number of elements.
 */
void fb_inv_sim(fb_t * c, fb_t * a, int n);

/**
 * Solves a quadratic equation for a, Tr(a) = 0 by repeated squarings and
 * additions.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to solve.
 */
void fb_slv_basic(fb_t c, fb_t a);

/**
 * Solves a quadratic equation for a, Tr(a) = 0 with precomputed half-traces.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to solve.
 */
void fb_slv_quick(fb_t c, fb_t a);

#endif /* !RELIC_FB_H */
