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
 * @file
 *
 * Implementation of tracing routines.
 *
 * @version $Id: relic_trace.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup relic
 */

#include <stdlib.h>
#include <stdio.h>
#include <stdarg.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_trace.h"

#ifdef TRACE
#include <execinfo.h>
#include <dlfcn.h>
#endif

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#ifdef TRACE
/**
 * Prints some spaces before the stack trace.
 */
#define FPRINTF fprintf(stderr, "%*s", core_ctx->trace+1, " "); fprintf
#else
/**
 * Prints with fprintf if tracing is disabled.
 */
#define FPRINTF fprintf
#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void trace_enter(void *this, void *from) {
	core_ctx->trace++;
#ifdef VERBS
	Dl_info info;

	dladdr(this, &info);
	if (info.dli_sname != NULL) {
		FPRINTF(stderr, "%d - running %s()\n", core_ctx->trace, info.dli_sname);
	}
#endif
	(void)this;
	(void)from;
}

void trace_exit(void *this, void *from) {
#ifdef VERBS
	Dl_info info;

	dladdr(this, &info);
	if (info.dli_sname != NULL) {
		FPRINTF(stderr, "%d - exiting %s()\n", core_ctx->trace, info.dli_sname);
	}
#endif
	if (core_ctx->trace > 0) {
		core_ctx->trace--;
	}
	(void)this;
	(void)from;
}
