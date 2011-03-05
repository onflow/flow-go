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
 * @defgroup ft Ternary field arithmetic.
 */

/**
 * @file
 *
 * Interface of the ternary field arithmetic module.
 *
 * @version $Id$
 * @ingroup fb
 */

#ifndef RELIC_FT_H
#define RELIC_FT_H

#include "relic_dv.h"
#include "relic_conf.h"
#include "relic_types.h"

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Precision in trits of a ternary field element.
 */
#define FT_TRITS 	((int)FT_POLYN)

/**
 * Precision in bits of a ternary field element considering that each
 * coefficient takes 2 bits to be properly represented.
 */
#define FT_BITS 	((int)2 * FT_POLYN)

/**
 * Size in bits of a digit.
 */
#define FT_DIGIT	((int)DIGIT)

/**
 * Logarithm of the digit size in base 2.
 */
#define FT_DIG_LOG	((int)DIGIT_LOG)

/**
 * Size in digits of a block sufficient to store a ternary field element.
 */
#define FT_DIGS		2 * ((int)((FT_TRITS)/(FT_DIGIT) + (FT_TRITS % FT_DIGIT > 0)))

/**
 * Size in bytes of a block sufficient to store a ternary field element.
 */
#define FT_BYTES 	(FT_DIGS * sizeof(dig_t))

/**
 * Finite field identifiers.
 */
enum {
	/** Pentanomial commonly used for implementing pairings. */
	TRINO_97 = 1,
	/** Pentanomial commonly used for implementing pairings. */
	PENTA_509,
};

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a ternary field element.
 */
#if ALLOC == AUTO
typedef align dig_t ft_t[FT_DIGS + PADDING(FT_BYTES)/sizeof(dig_t)];
#else
typedef dig_t *ft_t;
#endif

/**
 * Represents a ternary field element with automatic memory allocation.
 */
typedef align dig_t ft_st[FT_DIGS + PADDING(FT_BYTES)/sizeof(dig_t)];

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a ternary field element with a null value.
 *
 * @param[out] A			- the ternary field element to initialize.
 */
#if ALLOC == AUTO
#define ft_null(A)			/* empty */
#else
#define ft_null(A)			A = NULL;
#endif

/**
 * Calls a function to allocate a ternary field element.
 *
 * @param[out] A			- the new ternary field element.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define ft_new(A)			dv_new_dynam((dv_t *)&(A), FT_DIGS)
#elif ALLOC == STATIC
#define ft_new(A)			dv_new_statc((dv_t *)&(A), FT_DIGS)
#elif ALLOC == AUTO
#define ft_new(A)			/* empty */
#elif ALLOC == STACK
#define ft_new(A)															\
	A = (dig_t *)alloca(FT_BYTES + PADDING(FT_BYTES));						\
	A = (dig_t *)ALIGNED(A);												\

#endif

/**
 * Calls a function to free a ternary field element.
 *
 * @param[out] A			- the ternary field element to clean and free.
 */
#if ALLOC == DYNAMIC
#define ft_free(A)			dv_free_dynam((dv_t *)&(A))
#elif ALLOC == STATIC
#define ft_free(A)			dv_free_statc((dv_t *)&(A))
#elif ALLOC == AUTO
#define ft_free(A)			/* empty */
#elif ALLOC == STACK
#define ft_free(A)			A = NULL;
#endif

/**
 * Multiples two ternary field elements. Computes c = a * b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first ternary field element to multiply.
 * @param[in] B				- the second ternary field element to multiply.
 */
#if FT_KARAT > 0
#define ft_mul(C, A, B)	ft_mul_karat(C, A, B)
#elif FT_MUL == BASIC
#define ft_mul(C, A, B)	ft_mul_basic(C, A, B)
#elif FT_MUL == INTEG
#define ft_mul(C, A, B)	ft_mul_integ(C, A, B)
#elif FT_MUL == LODAH
#define ft_mul(C, A, B)	ft_mul_lodah(C, A, B)
#endif

/**
 * Cubes a ternary field element. Computes c = a * a.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the binary field element to square.
 */
#if FT_CUB == BASIC
#define ft_cub(C, A)	ft_cub_basic(C, A)
#elif FT_CUB == TABLE
#define ft_cub(C, A)	ft_cub_table(C, A)
#elif FT_CUB == INTEG
#define ft_cub(C, A)	ft_cub_integ(C, A)
#endif

/**
 * Reduces a multiplication result modulo a ternary irreducible polynomial.
 * Computes c = a mod f(z).
 *
 * @param[out] C			- the result.
 * @param[in] A				- the multiplication result to reduce.
 */
#if FT_RDC == BASIC
#define ft_rdc_mul(C, A)	ft_rdc_mul_basic(C, A)
#elif FT_RDC == QUICK
#define ft_rdc_mul(C, A)	ft_rdc_mul_quick(C, A)
#endif

/**
 * Reduces a cubing result modulo a ternary irreducible polynomial.
 * Computes c = a mod f(z).
 *
 * @param[out] C			- the result.
 * @param[in] A				- the multiplication result to reduce.
 */
#if FT_RDC == BASIC
#define ft_rdc_cub(C, A)	ft_rdc_cub_basic(C, A)
#elif FT_RDC == QUICK
#define ft_rdc_cub(C, A)	ft_rdc_cub_quick(C, A)
#endif

/**
 * Extracts the cube root of a ternary field element. Computes c = a^(1/3).
 *
 * @param[out] C			- the result.
 * @param[in] A				- the ternary field element.
 */
#if FT_CRT == BASIC
#define ft_crt(C, A)	ft_crt_basic(C, A)
#elif FT_CRT == QUICK
#define ft_crt(C, A)	ft_crt_quick(C, A)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the ternary field arithmetic layer.
 */
void ft_poly_init(void);

/**
 * Finalizes the ternary field arithmetic layer.
 */
void ft_poly_clean(void);

/**
 * Returns the irreducible polynomial f(z) configured for the ternary field.
 *
 * @return the irreducible polynomial.
 */
dig_t *ft_poly_get(void);

/**
 * Configures the irreducible polynomial of the ternary field.
 *
 * @param[in] f				- the new irreducible polynomial.
 */
void ft_poly_set(ft_t f);

/**
 * Configures a trinomial as the irreducible polynomial by its non-zero
 * coefficients. The remaining coefficient is FT_TRITS.
 *
 * @param[in] a				- the second coefficient.
 * @param[in] b				- the third coefficiet.
 */
void ft_poly_set_trino(int a, int b);

/**
 * Configures a pentanomial as the ternary field modulo by its non-zero
 * coefficients. The remaining coefficients is FT_TRITS.
 *
 * @param[in] a				- the second coefficient.
 * @param[in] b				- the third coefficient.
 * @param[in] c				- the fourth coefficient.
 * @param[in] d				- the last coefficient.
 */
void ft_poly_set_penta(int a, int b, int c, int d);

/**
 * Returns the cube root of z.
 *
 * @return the cube root of z.
 */
dig_t *ft_poly_get_crz(void);

/**
 * Returns the square of the cube root of z.
 *
 * @return the square of the cube root of z.
 */
dig_t *ft_poly_get_srz(void);

/**
 * Returns the square of the cube root of z in sparse form.
 *
 * @param[out] len			- the number of terms in the squared cube root of z.
 * @return the square of the cube root of z in sparse form.
 */
int *ft_poly_get_srz_spars(int *len);

/**
 * Returns the cube root of z.
 *
 * @return the cube root of z.
 */
dig_t *ft_poly_get_crz(void);

/**
 * Returns the cube root of z in sparse form.
 *
 * @param[out] len			- the number of terms in the cube root of z.
 * @return the cube root of z in sparse form.
 */
int *ft_poly_get_crz_spars(int *len);

/**
 * Returns sqrt(z) * (i represented as a polynomial).
 *
 * @return the precomputed result.
 */
dig_t *ft_poly_get_tab_srz(int i);

/**
 * Returns the consecutive squaring (i as a polynomial * z^4j)^{2^k}.
 *
 * @return the precomputed result.
 */
dig_t *ft_poly_get_tab_sqr(int i, int j, int k);

/**
 * Returns the non-zero coefficients of the configured trinomial or pentanomial.
 * If c is -1, the irreducible polynomial configured is a trinomial.
 * The remaining coefficient is FT_TRITS.
 *
 * @param[out] a			- the second coefficient.
 * @param[out] b			- the third coefficient.
 * @param[out] c			- the fourth coefficient.
 * @param[out] d			- the last coefficient.
 */
void ft_poly_get_rdc(int *a, int *b, int *c, int *d);

/**
 * Returns the non-zero bits used to compute the trace function. The -1
 * coefficient is the last coefficient.
 *
 * @param[out] a			- the first coefficient.
 * @param[out] b			- the second coefficient.
 * @param[out] c			- the third coefficient.
 */
void ft_poly_get_trc(int *a, int *b, int *c);

/**
 * Returns the half-trace of z^i, for odd i.
 *
 * @return the precomputed half-trace.
 */
dig_t *ft_poly_get_slv(int i, int j);

/**
 * Assigns a standard irreducible polynomial as modulo of the ternary field.
 *
 * @param[in] param			- the standardized polynomial identifier.
 */
void ft_param_set(int param);

/**
 * Configures some finite field parameters for the current security level.
 */
void ft_param_set_any(void);

/**
 * Prints the currently configured irreducible polynomial.
 */
void ft_param_print(void);

/**
 * Adds a ternary field element and the irreducible polynomial. Computes
 * c = a + f(z).
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the ternary field element.
 */
void ft_poly_add(ft_t c, ft_t a);

/**
 * Subtracts the irreducible polynomial from a ternary field element. Computes
 * c = a - f(z).
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the ternary field element.
 */
void ft_poly_sub(ft_t c, ft_t a);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to copy.
 */
void ft_copy(ft_t c, ft_t a);

/**
 * Negates a ternary field element.
 *
 * @param[out] c			- the result.
 * @param[out] a			- the ternary field element to negate.
 */
void ft_neg(ft_t c, ft_t a);

/**
 * Assigns zero to a ternary field element.
 *
 * @param[out] a			- the ternary field element to assign.
 */
void ft_zero(ft_t a);

/**
 * Tests if a ternary field element is zero or not.
 *
 * @param[in] a				- the ternary field element to test.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int ft_is_zero(ft_t a);

/**
 * Tests the bit in the given position on a multiple precision integer.
 *
 * @param[in] a				- the ternary field element to test.
 * @param[in] bit			- the bit position.
 * @return 0 is the bit is zero, not zero otherwise.
 */
int ft_test_bit(ft_t a, int bit);

/**
 * Reads the bit stored in the given position on a ternary field element.
 *
 * @param[in] a				- the ternary field element.
 * @param[in] bit			- the bit position.
 * @return the bit value.
 */
int ft_get_bit(ft_t a, int bit);

/**
 * Stores a bit in a given position on a ternary field element.
 *
 * @param[out] a			- the ternary field element.
 * @param[in] bit			- the bit position.
 * @param[in] value			- the bit value.
 */
void ft_set_bit(ft_t a, int bit, int value);

/**
 * Tests if the trit in a given position is non-zero on a ternary field element.
 *
 * @param[in] a				- the ternary field element to test.
 * @param[in] trit			- the trit position.
 * @return 0 is the trit is zero, not zero otherwise.
 */
int ft_test_trit(ft_t a, int bit);

/**
 * Reads the trit stored in the given position on a ternary field element.
 *
 * @param[in] a				- the ternary field element.
 * @param[in] trit			- the trit position.
 * @return the trit value.
 */
int ft_get_trit(ft_t a, int trit);

/**
 * Stores a trit in a given position on a ternary field element.
 *
 * @param[out] a			- the ternary field element.
 * @param[in] trit			- the trit position.
 * @param[in] value			- the trit value.
 */
void ft_set_trit(ft_t a, int trit, int value);

/**
 * Assigns a small positive polynomial to a ternary field element.
 *
 * The degree of the polynomial must be smaller than FT_DIGIT.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the small polynomial to assign.
 */
void ft_set_dig(ft_t c, dig_t a);

/**
 * Returns the position of the most significant bit on a ternary field element.
 *
 * @param[in] a				- the ternary field element.
 * @return the number of bits.
 */
int ft_bits(ft_t a);

/**
 * Returns the position of the most significant trit on a ternary field element.
 *
 * @param[in] a				- the ternary field element.
 * @return the number of trits.
 */
int ft_trits(ft_t a);

/**
 * Assigns a random value to a ternary field element.
 *
 * @param[out] a			- the ternary field element to assign.
 */
void ft_rand(ft_t a);

/**
 * Prints a ternary field element to standard output.
 *
 * @param[in] a				- the ternary field element to print.
 */
void ft_print(ft_t a);

/**
 * Returns the number of digits in radix 3 necessary to store a ternary field
 * element.
 *
 * @param[out] size			- the result.
 * @param[in] a				- the ternary field element.
 */
void ft_size(int *size, ft_t a);

/**
 * Reads a ternary field element from a string in radix 3.
 *
 * @param[out] a			- the result.
 * @param[in] str			- the string.
 * @param[in] len			- the size of the string.
 */
void ft_read(ft_t a, const char *str, int len);

/**
 * Writes a ternary field element to a string in radix 3.
 *
 * @param[out] str			- the string.
 * @param[in] len			- the buffer capacity.
 * @param[in] a				- the ternary field element to write.
 * @throw ERR_NO_BUFFER		- if the buffer capacity is insufficient.
 */
void ft_write(char *str, int len, ft_t a);

/**
 * Returns the result of a comparison between a ternary field element
 * and a small ternary field element.
 *
 * @param[in] a				- the ternary field element.
 * @param[in] b				- the small ternary field element.
 * @return FT_LT if a < b, FT_EQ if a == b and FT_GT if a > b.
 */
int ft_cmp_dig(ft_t a, dig_t b);

/**
 * Returns the result of a comparison between two ternary field elements.
 *
 * @param[in] a				- the first ternary field element.
 * @param[in] b				- the second ternary field element.
 * @return FT_LT if a < b, FT_EQ if a == b and FT_GT if a > b.
 */
int ft_cmp(ft_t a, ft_t b);

/**
 * Adds two ternary field elements. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first ternary field element to add.
 * @param[in] b				- the second ternary field element to add.
 */
void ft_add(ft_t c, ft_t a, ft_t b);

/**
 * Adds a ternary field element and a small ternary field element.
 * Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to add.
 * @param[in] b				- the small ternary field element to add.
 */
void ft_add_dig(ft_t c, ft_t a, dig_t b);

/**
 * Subtracts a ternary field element from another. Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element.
 * @param[in] b				- the ternary field element to subtract.
 */
void ft_sub(ft_t c, ft_t a, ft_t b);

/**
 * Subtracts a small ternary field element from a ternary field element.
 * Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element.
 * @param[in] b				- the small ternary field element to subtract.
 */
void ft_sub_dig(ft_t c, ft_t a, dig_t b);

/**
 * Multiples two ternary field elements using Shift-and-add multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first ternary field element to multiply.
 * @param[in] b				- the second ternary field element to multiply.
 */
void ft_mul_basic(ft_t c, ft_t a, ft_t b);

/**
 * Multiples two ternary field elements using multiplication integrated with
 * modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first ternary field element to multiply.
 * @param[in] b				- the second ternary field element to multiply.
 */
void ft_mul_integ(ft_t c, ft_t a, ft_t b);

/**
 * Multiples two ternary field elements using López-Dahab multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first ternary field element to multiply.
 * @param[in] b				- the second ternary field element to multiply.
 */
void ft_mul_lodah(ft_t c, ft_t a, ft_t b);

/**
 * Cubes a ternary field element using three multiplications.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to cube.
 */
void ft_cub_basic(ft_t c, ft_t a);

/**
 * Cubes a ternary field element using table-based cubing.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to square.
 */
void ft_cub_table(ft_t c, ft_t a);

/**
 * Cubes a ternary field element with integrated modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to square.
 */
void ft_cub_integ(ft_t c, ft_t a);

/**
 * Shifts a ternary field element to the left. Computes
 * c = a * z^trits mod f(z).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to shift.
 * @param[in] trits			- the number of trits to shift.
 */
void ft_lsh(ft_t c, ft_t a, int trits);

/**
* Shifts a ternary field element to the right. Computes c = a / (z^trits).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to shift.
 * @param[in] trits			- the number of trits to shift.
 */
void ft_rsh(ft_t c, ft_t a, int trits);

/**
 * Reduces a multiplication result modulo an irreducible polynomial using
 * shift-and-add modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void ft_rdc_mul_basic(ft_t c, dv_t a);

/**
 * Reduces a multiplication result modulo a trinomial or pentanomial.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiplication result to reduce.
 */
void ft_rdc_mul_quick(ft_t c, dv_t a);

/**
 * Reduces a cubing result modulo an irreducible polynomial using
 * shift-and-add modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cubing result to reduce.
 */
void ft_rdc_cub_basic(ft_t c, dv_t a);

/**
 * Reduces a cubing result modulo a trinomial or pentanomial.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cubing result to reduce.
 */
void ft_rdc_cub_quick(ft_t c, dv_t a);

/**
 * Extracts the cube root of a ternary field element using repeated cubing.
 * Computes c = a^{1/3}.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to take a cube root.
 */
void ft_crt_basic(ft_t c, ft_t a);

/**
 * Extracts the cube root of a ternary field element using a fast cube root
 * extraction algorithm.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the ternary field element to take a cube root.
 */
void ft_crt_quick(ft_t c, ft_t a);

#endif /* !RELIC_FT_H */
