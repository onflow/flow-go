/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * @defgroup relic Core functions.
 */

/**
 * @file
 *
 * Interface of the library core functions.
 *
 * @version $Id$
 * @ingroup relic
 */

#ifndef RELIC_CORE_H
#define RELIC_CORE_H

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_error.h"
#include "relic_bn.h"
#include "relic_ep.h"
#include "relic_conf.h"
#include "relic_rand.h"
#include "relic_pool.h"

#ifdef MULTI
#include <omp.h>
#include <math.h>
#endif

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Indicates that the function executed correctly.
 */
#define STS_OK			0

/**
 * Indicates that an error occurred during the function execution.
 */
#define STS_ERR			1

/**
 * Indicates that a comparison returned that the first argument was lesser than
 * the second argument.
 */
#define CMP_LT			-1

/**
 * Indicates that a comparison returned that the first argument was equal to
 * the second argument.
 */
#define CMP_EQ			0

/**
 * Indicates that a comparison returned that the first argument was greater than
 * the second argument.
 */
#define CMP_GT			1

/**
 * Indicates that two incomparable elements are not equal.
 */
#define CMP_NE			2

/**
 * Optimization identifer for the case where a coefficient is 0.
 */
#define OPT_ZERO		0

/**
 * Optimization identifer for the case where a coefficient is 1.
 */
#define OPT_ONE			1

/**
 * Optimization identifer for the case where a coefficient is 1.
 */
#define OPT_TWO			2

/**
 * Optimization identifer for the case where a coefficient is small.
 */
#define OPT_DIGIT		3

/**
 * Optimization identifier for the case where a coefficient is -3.
 */
#define OPT_MINUS3		4

/**
 * Optimization identifier for the case where the coefficient is random
 */
#define OPT_NONE		5

/**
 * Maximum number of terms to describe a sparse object.
 */
#define MAX_TERMS		16

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Library context.
 */
typedef struct _ctx_t {
	/** The value returned by the last call, can be STS_OK or STS_ERR. */
	int code;

#ifdef CHECK
	/** The state of the last error caught. */
	sts_t *last;
	/** The error message respective to the last error. */
	char *reason[ERR_MAX];
	/** A flag to indicate if the last error was already caught. */
	int caught;
#endif /* CHECK */

#if defined(CHECK) || defined(TRACE)
	/** The current trace size. */
	int trace;
#endif /* CHECK || TRACE */

#if ALLOC == STATIC
	/** The static pool of digit vectors. */
	pool_t pool[POOL_SIZE];
	/** The index of the next free digit vector in the pool. */
	int next;
#endif /* ALLOC == STATIC */

#ifdef WITH_FP
	/** Currently configured prime field identifier. */
	int prime_id;
	/** Currently configured prime modulus. */
	bn_st prime;
	/** Prime modulus modulo 8. */
	dig_t mod8;
	/** Value derived from the prime used for modular reduction. */
	dig_t u;
	/** Value (R^2 mod p) for converting small integers to Montgomery form. */
	bn_st conv;
	/** Value of constant one in Montgomery form. */
	bn_st one;
	/** Quadratic non-residue. */
	int qnr;
	/** Cubic non-residue. */
	int cnr;
	/** Sparse representation of prime modulus. */
	int sps[MAX_TERMS + 1];
	/** Length of sparse prime representation. */
	int len;
	/** Sparse representation of parameter used to generate prime. */
	int var[MAX_TERMS + 1];
#endif

	/** Internal state of the PRNG. */
	unsigned char rand[RAND_SIZE];
} ctx_t;

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the library.
 *
 * @return STS_OK if no error occurs, STS_ERR otherwise.
 */
int core_init(void);

/**
 * Finalizes the library.
 *
 * @return STS_OK if no error occurs, STS_ERR otherwise.
 */
int core_clean(void);

/**
 * Returns a pointer to the current library context.
 *
 * @return a pointer to the library context.
 */
ctx_t *core_get(void);

/**
 * Switched the library context to a new context.
 *
 * @param[in] ctx					- the new library context.
 */
void core_set(ctx_t *ctx);

#endif /* !RELIC_CORE_H */
