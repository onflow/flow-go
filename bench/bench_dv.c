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
 * @file
 *
 * Benchmarks for the temporary vector memory-management routines.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void dv_new_impl(dv_t *a) {
	dv_new(*a);
}

static void memory(void) {
	dv_t a[BENCH + 1] = { NULL };
	dv_t *tmpa;

	BENCH_BEGIN("dv_new") {
		tmpa = a;
		BENCH_ADD(dv_new_impl(tmpa++));
		for (int j = 0; j <= BENCH; j++) {
			dv_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("dv_free") {
		for (int j = 0; j <= BENCH; j++) {
			dv_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(dv_free(*(tmpa++)));
	}
	BENCH_END;
}

int main(void) {
	core_init();
	conf_print();
	util_print_banner("Benchmarks for the DV module:\n", 0);
	util_print_banner("Utilities:\n", 0);
	memory();
	core_clean();
	return 0;
}
