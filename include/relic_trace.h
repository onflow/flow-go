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
 * Interface of the tracing functions.
 *
 * @version $Id: relic_trace.h 3 2009-04-08 01:05:51Z dfaranha $
 * @ingroup relic
 */

#ifndef RELIC_TRACE_H
#define RELIC_TRACE_H

#ifdef TRACE

/**
 * Renames the tracing routine called when a function is called to the compiler
 * specific function.
 */
#define trace_enter	__cyg_profile_func_enter

/**
 * Renames the tracing routine called when a function returns to the compiler
 * specific function.
 */
#define trace_exit	__cyg_profile_func_exit

/**
 * Prints the name of the function begin called for tracing purposes.
 *
 * @param[in] this		- the function address.
 * @param[in] from		- address of the caller function.
 */
void trace_enter(void *this, void *from)
		__attribute__ ((no_instrument_function));

/**
 * Prints the name of the function begin returned for tracing purposes.
 *
 * @param[in] this		- the function address.
 * @param[in] from		- address of the caller function.
 */
void trace_exit(void *this, void *from)
		__attribute__ ((no_instrument_function));

#endif

#endif /* !RELIC_TRACE_H */
