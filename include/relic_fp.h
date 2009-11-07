/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * @defgroup fp Prime field arithmetic.
 */

/**
 * @file
 *
 * Interface of the prime field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp
 */

#ifndef RELIC_FP_H
#define RELIC_FP_H

#include "relic_bn.h"
#include "relic_dv.h"
#include "relic_types.h"

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Precision in bits of a prime field element.
 */
#define FP_BITS 		((int)FP_PRIME)

/**
 * Size in bits of a digit.
 */
#define FP_DIGIT		((int)DIGIT)

/**
 * Logarithm of the digit size in base 2.
 */
#define FP_DIG_LOG		((int)DIGIT_LOG)

/**
 * Size in digits of a block sufficient to store a prime field element.
 */
#define FP_DIGS	((int)((FP_BITS)/(FP_DIGIT) + (FP_BITS % FP_DIGIT > 0)))

/*
 * Finite field identifiers.
 */
enum {
	/** NIST 192-bit fast reduction prime. */
	NIST_192 = 1,
	/** NIST 224-bit fast reduction polynomial. */
	NIST_224 = 2,
	/** NIST 256-bit fast reduction polynomial. */
	NIST_256 = 3,
	/** NIST 384-bit fast reduction polynomial. */
	NIST_384 = 4,
	/** NIST 521-bit fast reduction polynomial. */
	NIST_521 = 5,
	/** 256-bit prime provided in Nogami et al. for use with BN curves. */
	BNN_256 = 6,
	/** 256-bit prime provided in Nogami et al. for use with BN curves. */
	BNP_256 = 7,
};

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a prime field element.
 *
 * A field element is represented as a digit vector. These digits are organized
 * in little-endian format, that is, the least significant digits are
 * stored in the first positions of the vector.
 */
typedef dig_t *fp_t;

/**
 * Represents a prime field element with automatic memory allocation.
 */
typedef align dig_t fp_st[FP_DIGS];

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a binary field element with a null value.
 *
 * @param[out] A			- the binary field element to initialize.
 */
#define fp_null(A)			A = NULL;

/**
 * Calls a function to allocate and initialize a prime field element.
 *
 * @param[out] A			- the new prime field element.
 */
#if ALLOC == DYNAMIC
#define fp_new(A)			dv_new_dynam((dv_t *)&(A), FP_DIGS)
#elif ALLOC == STATIC
#define fp_new(A)			dv_new_statc((dv_t *)&(A), FP_DIGS)
#elif ALLOC == STACK
#define fp_new(A)															\
	A = (dig_t *)alloca(FP_DIGS * sizeof(dig_t) + ALIGN); ALIGNED(A);		\

#endif

/**
 * Calls a function to clean and free a prime field element.
 *
 * @param[out] A			- the prime field element to clean and free.
 */
#if ALLOC == DYNAMIC
#define fp_free(A)			dv_free_dynam((dv_t *)&(A))
#elif ALLOC == STATIC
#define fp_free(A)			dv_free_statc((dv_t *)&(A))
#elif ALLOC == STACK
#define fp_free(A)			A = NULL;
#endif

/**
 * Multiples two prime field elements. Computes c = a * b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first prime field element.
 * @param[in] B				- the second prime field element.
 */
#if FP_KARAT > 0
#define fp_mul(C, A, B)		fp_mul_karat(C, A, B)
#elif FP_MUL == BASIC
#define fp_mul(C, A, B)		fp_mul_basic(C, A, B)
#elif FP_MUL == COMBA
#define fp_mul(C, A, B)		fp_mul_comba(C, A, B)
#elif FP_MUL == INTEG
#define fp_mul(C, A, B)		fp_mul_integ(C, A, B)
#endif

/**
 * Squares a prime field element. Computes c = a * a.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the prime field element to square.
 */
#if FP_KARAT > 0
#define fp_sqr(C, A)		fp_sqr_karat(C, A)
#elif FP_SQR == BASIC
#define fp_sqr(C, A)		fp_sqr_basic(C, A)
#elif FP_SQR == COMBA
#define fp_sqr(C, A)		fp_sqr_comba(C, A)
#elif FP_SQR == INTEG
#define fp_sqr(C, A)		fp_sqr_integ(C, A)
#endif

/**
 * Reduces a multiplication result modulo a prime field order. Computes
 * c = a mod m.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the multiplication result to reduce.
 */
#if FP_RDC == BASIC
#define fp_rdc(C, A)		fp_rdc_basic(C, A)
#elif FP_RDC == MONTY
#define fp_rdc(C, A)		fp_rdc_monty(C, A)
#elif FP_RDC == QUICK
#define fp_rdc(C, A)		fp_rdc_quick(C, A)
#endif

/**
 * Reduces a multiplication result modulo a prime field order using Montgomery
 * modular reduction.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the multiplication result to reduce.
 */
#if FP_MUL == BASIC
#define fp_rdc_monty(C, A)	fp_rdc_monty_basic(C, A)
#elif  FP_MUL == COMBA
#define fp_rdc_monty(C, A)	fp_rdc_monty_comba(C, A)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the prime field arithmetic layer.
 */
void fp_prime_init(void);

/**
 * Finalizes the prime field arithmetic layer.
 */
void fp_prime_clean(void);

/**
 * Returns the order of the prime field.
 *
 * @return the order of the prime field.
 */
dig_t *fp_prime_get(void);

/**
 * Returns the additional value used for modular reduction.
 *
 * @return the additional value used for modular reduction.
 */
dig_t *fp_prime_get_rdc(void);

/**
 * Returns the additional value used for conversion from multiple precision
 * integer to prime field element.
 *
 * @return the additional value used for importing integers.
 */
dig_t *fp_prime_get_conv(void);

/**
 * Returns the prime stored in special form. The most significant bit is
 * FP_BITS.
 *
 * @param[out] len		- the number of returned bits, can be NULL.
 *
 * @return the prime represented by it non-zero bits.
 */
int *fp_prime_get_spars(int *len);

/**
 * Returns a non-quadratic residue in the prime field.
 *
 * @return the non-quadratic residue.
 */
int fp_prime_get_qnr(void);

/**
 * Returns a non-cubic residue in the prime field.
 *
 * @return the non-cubic residue.
 */
int fp_prime_get_cnr(void);

/**
 * Returns the result of prime order mod 8.
 *
 * @return the result of prime order mod 8.
 */
dig_t *fp_prime_get_mod8(void);

/**
 * Assigns the order of the prime field to a non-sparse prime p.
 *
 * @param[in] p			- the new prime field order.
 */
void fp_prime_set_dense(bn_t p);

/**
 * Assigns the order of the prime field to a special form prime p.
 *
 * @param[in] p			- the new prime field order.
 */
void fp_prime_set_spars(int *spars, int len);

/**
 * Assigns a prime modulus based on its identifier.
 */
void fp_param_set(int param);

/**
 * Assigns any pre-defined parameter as the prime modulus.
 */
void fp_param_set_any(void);

/**
 * Prints the currently configured prime modulus.
 */
void fp_param_print(void);
/**
 * Copies the second argument to the first argument.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to copy.
 */
void fp_copy(fp_t c, fp_t a);

/**
 * Negates a prime field element.
 *
 * @param[out] c			- the result.
 * @param[out] a			- the prime field element to negate.
 */
void fp_neg(fp_t c, fp_t a);

/**
 * Assigns zero to a prime field element.
 *
 * @param[out] a			- the prime field element to asign.
 */
void fp_zero(fp_t a);

/**
 * Tests if a prime field element is zero or not.
 *
 * @param[in] a				- the prime field element to test.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int fp_is_zero(fp_t a);

/**
 * Tests if a prime field element is even or odd.
 *
 * @param[in] a				- the prime field element to test.
 * @return 1 if the argument is even, 0 otherwise.
 */
int fp_is_even(fp_t a);

/**
 * Tests the bit in the given position on a multiple precision integer.
 *
 * @param[in] a				- the prime field element to test.
 * @param[in] bit			- the bit position.
 * @return 0 is the bit is zero, not zero otherwise.
 */
int fp_test_bit(fp_t a, int bit);

/**
 * Reads the bit stored in the given position on a prime field element.
 *
 * @param[in] a				- the prime field element.
 * @param[in] bit			- the bit position.
 * @return the bit value.
 */
int fp_get_bit(fp_t a, int bit);

/**
 * Stores a bit in a given position on a prime field element.
 *
 * @param[out] a			- the prime field element.
 * @param[in] bit			- the bit position.
 * @param[in] value			- the bit value.
 */
void fp_set_bit(fp_t a, int bit, int value);

/**
 * Assigns a small positive constant to a prime field element.
 *
 * The constant must fit on a multiple precision digit, or dig_t type using
 * only the number of bits specified on FP_DIGIT.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the constant to assign.
 */
void fp_set_dig(fp_t c, dig_t a);

/**
 * Returns the number of bits of a prime field element.
 *
 * @param[in] a				- the prime field element.
 * @return the number of bits.
 */
int fp_bits(fp_t a);

/**
 * Assigns a random value to a prime field element.
 *
 * @param[out] a			- the prime field element to assign.
 */
void fp_rand(fp_t a);

/**
 * Prints a prime field element to standard output.
 *
 * @param[in] a				- the prime field element to print.
 */
void fp_print(fp_t a);

/**
 * Returns the number of digits in radix necessary to store a multiple precision
 * integer. The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] size			- the result.
 * @param[in] a				- the prime field element.
 * @param[in] radix			- the radix.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fp_size(int *size, fp_t a, int radix);

/**
 * Reads a prime field element from a string in a given radix. The radix must
 * be a power of 2 included in the interval [2, 64].
 *
 * @param[out] a			- the result.
 * @param[in] str			- the string.
 * @param[in] len			- the size of the string.
 * @param[in] radix			- the radix.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fp_read(fp_t a, const char *str, int len, int radix);

/**
 * Writes a prime field element to a string in a given radix. The radix must
 * be a power of 2 included in the interval [2, 64].
 *
 * @param[out] str			- the string.
 * @param[in] len			- the buffer capacity.
 * @param[in] a				- the prime field element to write.
 * @param[in] radix			- the radix.
 * @throw ERR_BUFFER		- if the buffer capacity is insufficient.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
void fp_write(char *str, int len, fp_t a, int radix);

/**
 * Returns the result of a signed comparison between a prime field element
 * and a digit.
 *
 * @param[in] a				- the prime field element.
 * @param[in] b				- the digit.
 * @return FP_LT if a < b, FP_EQ if a == b and FP_GT if a > b.
 */
int fp_cmp_dig(fp_t a, dig_t b);

/**
 * Returns the result of a comparison between two prime field elements.
 *
 * @param[in] a				- the first prime field element.
 * @param[in] b				- the second prime field element.
 * @return FP_LT if a < b, FP_EQ if a == b and FP_GT if a > b.
 */
int fp_cmp(fp_t a, fp_t b);

/**
 * Adds two prime field elements. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to add.
 * @param[in] b				- the second prime field element to add.
 */
void fp_add(fp_t c, fp_t a, fp_t b);

/**
 * Adds a prime field element and a digit. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to add.
 * @param[in] b				- the digit to add.
 */
void fp_add_dig(fp_t c, fp_t a, dig_t b);

/**
 * Subtracts a prime field element from another. Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element.
 * @param[in] b				- the prime field element to subtract.
 */
void fp_sub(fp_t c, fp_t a, fp_t b);

/**
 * Subtracts a digit from a prime field element. Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element.
 * @param[in] b				- the digit to subtract.
 */
void fp_sub_dig(fp_t c, fp_t a, dig_t b);

/**
 * Multiples two prime field elements using Schoolbook multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to multiply.
 * @param[in] b				- the second prime field element to multiply.
 */
void fp_mul_basic(fp_t c, fp_t a, fp_t b);

/**
 * Multiples two prime field elements using Comba multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to multiply.
 * @param[in] b				- the second prime field element to multiply.
 */
void fp_mul_comba(fp_t c, fp_t a, fp_t b);

/**
 * Multiples two prime field elements using multiplication integrated with
 * modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to multiply.
 * @param[in] b				- the second prime field element to multiply.
 */
void fp_mul_integ(fp_t c, fp_t a, fp_t b);

/**
 * Multiples two prime field elements using Karatsuba multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first prime field element to multiply.
 * @param[in] b				- the second prime field element to multiply.
 */
void fp_mul_karat(fp_t c, fp_t a, fp_t b);

/**
 * Multiplies a prime field element by a digit. Computes c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element.
 * @param[in] b				- the digit to multiply.
 */
void fp_mul_dig(fp_t c, fp_t a, dig_t b);

/**
 * Squares a prime field element using Schoolbook squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to square.
 */
void fp_sqr_basic(fp_t c, fp_t a);

/**
 * Squares a prime field element using Comba squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to square.
 */
void fp_sqr_comba(fp_t c, fp_t a);

/**
 * Squares two prime field elements using squaring integrated with
 * modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to square.
 */
void fp_sqr_integ(fp_t c, fp_t a);

/**
 * Squares a prime field element using Karatsuba squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to square.
 */
void fp_sqr_karat(fp_t c, fp_t a);

/**
 * Multiplies a prime field element by 2. Computes c = 2 * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to multiply by 2.
 */
void fp_dbl(fp_t c, fp_t a);

/**
 * Divides a prime field element by 2. Computes c = floor(a / 2).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to divide by 2.
 */
void fp_hlv(fp_t c, fp_t a);

/**
 * Shifts a prime field element number to the left. Computes
 * c = a * 2^bits.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to shift.
 * @param[in] bits			- the number of bits to shift.
 */
void fp_lsh(fp_t c, fp_t a, int bits);

/**
 * Shifts a prime field element to the right. Computes c = floor(a / 2^bits).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to shift.
 * @param[in] bits			- the number of bits to shift.
 */
void fp_rsh(fp_t c, fp_t a, int bits);

/**
 * Inverts a prime field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to inver.
 */
void fp_inv(fp_t c, fp_t a);

/**
 * Reduces a multiplication result modulo the prime field modulo using
 * division-based reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fp_rdc_basic(fp_t c, dv_t a);

/**
 * Reduces a multiplication result modulo the prime field order using Shoolbook
 * Montgomery reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fp_rdc_monty_basic(fp_t c, dv_t a);

/**
 * Reduces a multiplication result modulo the prime field order using Comba
 * Montgomery reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fp_rdc_monty_comba(fp_t c, dv_t a);

/**
 * Reduces a multiplication result modulo the prime field modulo using
 * fast reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void fp_rdc_quick(fp_t c, dv_t a);

/**
 * Imports a multiple precision integer as a prime field element, doing the
 * necessary conversion.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiple precision integer to import.
 */
void fp_prime_conv(fp_t c, bn_t a);

/**
 * Imports a single digit as a prime field element, doing the necessary
 * conversion.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit to import.
 */
void fp_prime_conv_dig(fp_t c, dig_t a);

/**
 * Exports a prime field element as a multiple precision integer, doing the
 * necessary conversion.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element to export.
 */
void fp_prime_back(bn_t c, fp_t a);

#endif /* !RELIC_FP_H */
