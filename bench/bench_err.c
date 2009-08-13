/*
 * Copyright 2007-2009 RELIC Project
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
 * Benchmarks for the error-management routines.
 *
 * @version $Id: bench_error.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

void dummy2(void) {

}

void dummy(void) {
	TRY {

	}
	CATCH_ANY {
	}
	FINALLY {
	}
}

void error(void) {
	BENCH_BEGIN("empty function") {
		BENCH_ADD(dummy2());
	}
	BENCH_END;

	BENCH_BEGIN("try-catch-finnaly") {
		BENCH_ADD(dummy());
	}
	BENCH_END;
}

int main(void) {
	core_init();
	conf_print();
	util_print_label("Benchmarks for the ERR module:\n", 0);
	error();
	core_clean();
}
