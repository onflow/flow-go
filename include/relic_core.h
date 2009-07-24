/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
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
 * @version $Id: relic_core.h 45 2009-07-04 23:45:48Z dfaranha $
 * @ingroup relic
 */

#ifndef RELIC_CORE_H
#define RELIC_CORE_H

#include "relic_error.h"
#include "relic_conf.h"

#ifdef MULTI
#include <omp.h>
#include <math.h>
#endif

/**
 * Library context.
 */
typedef struct _ctx_t {
	/** The value returned by the last call, can be STS_OK or STS_ERR. */
	int code;
#ifdef CHECK
	/** The state of the last error caught. */
	state_t *last;
	/** The error message respective to the last error. */
	char **reason;
	/** A flag to indicate if the last error was already caught. */
	int caught;
#endif /* CHECK */
#if defined(CHECK) || defined(TRACE)
	/** The current trace size. */
	int trace;
#endif /* CHECK || TRACE */
} ctx_t;

/**
 * The active library context.
 */
extern ctx_t core_ctx[1];

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

#endif /* !RELIC_CORE_H */
