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
 * @file
 *
 * Implementation of the low-level binary field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.arch atmega128

.global fb_rdcn_low

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30,r24				; Copy c to z
	movw	r28,r22				; Copy a to y
	movw	r26,r28				; copy a + 68 to x
	adiw	r26, 48
	adiw	r26, 20

	ld		r18, -x
	ldd		r19, y + 33
	ldd		r20, y + 34
	mov		r22, r18
	lsl		r22
	eor		r19, r22
	bst		r18, 7
	clr		r22
	bld		r22, 0
	eor		r20, r22
	std		y + 33, r19
	std		y + 34, r20

	ldd		r19, y+(67 - 20)
	eor		r19, r18
	std		y+(67 - 20), r19
	ldd		r19, y+(67 - 12)
	eor		r19, r18
	std		y+(67 - 12), r19
	ldd		r19, y+(67 - 8)
	eor		r19, r18
	std		y+(67 - 8), r19

	clr r24
	clr	r19

	ld		r18, -x
	ldd		r20, y+(66 - 20)
	eor		r20, r18
	std		y+(66 - 20), r20
	ldd		r21, y+(66 - 12)
	eor		r21, r18
	ldd		r20, y+(66 - 8)
	eor		r20, r18
	std		y+(66 - 8), r20
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(66 - 33)
	eor		r20, r24
	std		y+(66 - 33), r20

	ld		r18, -x
	ldd		r20, y+(66 - 21)
	eor		r20, r18
	std		y+(66 - 21), r20
	ldd		r22, y+(66 - 13)
	eor		r22, r18
	ldd		r20, y+(66 - 9)
	eor		r20, r18
	std		y+(66 - 9), r20
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(66 - 34)
	eor		r20, r25
	std		y+(66 - 34), r20

	ld		r18, -x
	ldd		r20, y+(64 - 20)
	eor		r20, r18
	std		y+(64 - 20), r20
	ldd		r23, y+(64 - 12)
	eor		r23, r18
	ldd		r20, y+(64 - 8)
	eor		r20, r18
	std		y+(64 - 8), r20
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(64 - 33)
	eor		r20, r24
	std		y+(64 - 33), r20

	ld		r18, -x
	ldd		r20, y+(64 - 21)
	eor		r20, r18
	std		y+(64 - 21), r20
	ldd		r1, y+(64 - 13)
	eor		r1, r18
	ldd		r20, y+(64 - 9)
	eor		r20, r18
	std		y+(64 - 9), r20
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(64 - 34)
	eor		r20, r25
	std		y+(64 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(62 - 8), r21
	ldd		r20, y+(62 - 20)
	eor		r20, r18
	std		y+(62 - 20), r20
	ldd		r21, y+(62 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(62 - 33)
	eor		r20, r24
	std		y+(62 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(62 - 9), r22
	ldd		r20, y+(62 - 21)
	eor		r20, r18
	std		y+(62 - 21), r20
	ldd		r22, y+(62 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(62 - 34)
	eor		r20, r25
	std		y+(62 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(60 - 8), r23
	ldd		r20, y+(60 - 20)
	eor		r20, r18
	std		y+(60 - 20), r20
	ldd		r23, y+(60 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(60 - 33)
	eor		r20, r24
	std		y+(60 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(60 - 9), r1
	ldd		r20, y+(60 - 21)
	eor		r20, r18
	std		y+(60 - 21), r20
	ldd		r1, y+(60 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(60 - 34)
	eor		r20, r25
	std		y+(60 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(58 - 8), r21
	ldd		r20, y+(58 - 20)
	eor		r20, r18
	std		y+(58 - 20), r20
	ldd		r21, y+(58 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(58 - 33)
	eor		r20, r24
	std		y+(58 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(58 - 9), r22
	ldd		r20, y+(58 - 21)
	eor		r20, r18
	std		y+(58 - 21), r20
	ldd		r22, y+(58 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(58 - 34)
	eor		r20, r25
	std		y+(58 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(56 - 8), r23
	ldd		r20, y+(56 - 20)
	eor		r20, r18
	std		y+(56 - 20), r20
	ldd		r23, y+(56 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(56 - 33)
	eor		r20, r24
	std		y+(56 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(56 - 9), r1
	ldd		r20, y+(56 - 21)
	eor		r20, r18
	std		y+(56 - 21), r20
	ldd		r1, y+(56 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(56 - 34)
	eor		r20, r25
	std		y+(56 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(54 - 8), r21
	ldd		r20, y+(54 - 20)
	eor		r20, r18
	std		y+(54 - 20), r20
	ldd		r21, y+(54 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(54 - 33)
	eor		r20, r24
	std		y+(54 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(54 - 9), r22
	ldd		r20, y+(54 - 21)
	eor		r20, r18
	std		y+(54 - 21), r20
	ldd		r22, y+(54 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(54 - 34)
	eor		r20, r25
	std		y+(54 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(52 - 8), r23
	ldd		r20, y+(52 - 20)
	eor		r20, r18
	std		y+(52 - 20), r20
	ldd		r23, y+(52 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(52 - 33)
	eor		r20, r24
	std		y+(52 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(52 - 9), r1
	ldd		r20, y+(52 - 21)
	eor		r20, r18
	std		y+(52 - 21), r20
	ldd		r1, y+(52 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(52 - 34)
	eor		r20, r25
	std		y+(52 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(50 - 8), r21
	ldd		r20, y+(50 - 20)
	eor		r20, r18
	std		y+(50 - 20), r20
	ldd		r21, y+(50 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(50 - 33)
	eor		r20, r24
	std		y+(50 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(50 - 9), r22
	ldd		r20, y+(50 - 21)
	eor		r20, r18
	std		y+(50 - 21), r20
	ldd		r22, y+(50 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(50 - 34)
	eor		r20, r25
	std		y+(50 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(48 - 8), r23
	ldd		r20, y+(48 - 20)
	eor		r20, r18
	std		y+(48 - 20), r20
	ldd		r23, y+(48 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(48 - 33)
	eor		r20, r24
	std		y+(48 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(48 - 9), r1
	ldd		r20, y+(48 - 21)
	eor		r20, r18
	std		y+(48 - 21), r20
	ldd		r1, y+(48 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(48 - 34)
	eor		r20, r25
	std		y+(48 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(46 - 8), r21
	ldd		r20, y+(46 - 20)
	eor		r20, r18
	std		y+(46 - 20), r20
	ldd		r21, y+(46 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(46 - 33)
	eor		r20, r24
	std		z+(46 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(46 - 9), r22
	ldd		r20, y+(46 - 21)
	eor		r20, r18
	std		y+(46 - 21), r20
	ldd		r22, y+(46 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(46 - 34)
	eor		r20, r25
	std		z+(46 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(44 - 8), r23
	ldd		r20, y+(44 - 20)
	eor		r20, r18
	std		y+(44 - 20), r20
	ldd		r23, y+(44 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(44 - 33)
	eor		r20, r24
	std		z+(44 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(44 - 9), r1
	ldd		r20, y+(44 - 21)
	eor		r20, r18
	std		y+(44 - 21), r20
	ldd		r1, y+(44 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(44 - 34)
	eor		r20, r25
	std		z+(44 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(42 - 8), r21
	ldd		r20, y+(42 - 20)
	eor		r20, r18
	std		y+(42 - 20), r20
	ldd		r21, y+(42 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(42 - 33)
	eor		r20, r24
	std		z+(42 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		z+(42 - 9), r22
	ldd		r20, y+(42 - 21)
	eor		r20, r18
	std		z+(42 - 21), r20
	ldd		r22, y+(42 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(42 - 34)
	eor		r20, r25
	std		z+(42 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		z+(40 - 8), r23
	ldd		r20, y+(40 - 20)
	eor		r20, r18
	std		z+(40 - 20), r20
	ldd		r23, y+(40 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(40 - 33)
	eor		r20, r24
	std		z+(40 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		z+(40 - 9), r1
	ldd		r20, y+(40 - 21)
	eor		r20, r18
	std		z+(40 - 21), r20
	ldd		r1, y+(40 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(40 - 34)
	eor		r20, r25
	std		z+(40 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		z+(38 - 8), r21
	ldd		r20, y+(38 - 20)
	eor		r20, r18
	std		z+(38 - 20), r20
	ldd		r21, y+(38 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(38 - 33)
	eor		r20, r24
	std		z+(38 - 33), r20

	ld		r18, -x
	ldd		r20, y+(38 - 21)
	eor		r20, r18
	std		z+(38 - 21), r20
	ldd		r20, y+(38 - 13)
	eor		r20, r18
	std		z+(38 - 13), r20
	eor		r22, r18
	std		z+(38 - 9), r22
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(38 - 34)
	eor		r20, r25
	std		z+(38 - 34), r20

	ld		r18, -x
	ldd		r20, y+(36 - 20)
	eor		r20, r18
	std		z+(36 - 20), r20
	ldd		r20, y+(36 - 12)
	eor		r20, r18
	std		z+(36 - 12), r20
	eor		r23, r18
	std		z+(36 - 8), r23
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(36 - 33)
	eor		r20, r24
	std		z+(36 - 33), r20

	ld		r18, -x
	ldd		r20, y+(36 - 21)
	eor		r20, r18
	std		z+(36 - 21), r20
	ldd		r20, y+(36 - 13)
	eor		r20, r18
	std		z+(36 - 13), r20
	eor		r1, r18
	std		z+(36 - 9), r1
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(36 - 34)
	eor		r20, r25
	std		z+(36 - 34), r20

	ldd		r18, y+34
	ldd		r20, y+(34 - 20)
	eor		r20, r18
	std		z+(34 - 20), r20
	ldd		r20, y+(34 - 12)
	eor		r20, r18
	std		z+(34 - 12), r20
	eor		r21, r18
	std		z+(34 - 8), r21
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	lsl		r18
	mov		r25, r18
	ldd		r18, y + 1
	eor		r18, r24
	std		z + 1, r18
	ldd		r18, y + 0
	eor		r18, r25
	std		z + 0, r18

	ldd		r25, z + 33
	mov		r24, r25
	andi	r24, 0x80
	tst		r24
	breq	end
	ldi		r19, 0x01
	ldi		r20, 0x80
	ldd		r18, z + 0
	eor		r18, r19
	std		z + 0, r18
	ldd		r18, z + 13
	eor		r18, r20
	std		z + 13, r18
	ldd		r18, z + 21
	eor		r18, r20
	std		z + 21, r18
	ldd		r18, z + 25
	eor		r18, r20
	std		z + 25, r18
	andi	r25, 0x7F
	std		z + 33, r25

end:
	clr		r1

	pop		r29
	pop		r28
	ret
