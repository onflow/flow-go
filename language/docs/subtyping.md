
## Resources

- Supertype: **Restricted Resource**:

  - **Not** `AnyResource`:

    - A restricted resource type `T{Us}`
      is a subtype of a restricted resource type `V{Ws}`:

      - When `T != AnyResource`: if `T == V`.

        `Us` and `Ws` do *not* have to be subsets:
        The owner of the resource may freely restrict and unrestrict the resource.

        - Static: Yes
        - Dynamic: Yes

      - When `T == AnyResource`: if the run-time type is `V`.

        - Static: No
        - Dynamic: Yes

    - An unrestricted resource type `T`
      is a subtype of a restricted resource type `U{Vs}`:

      - When `T != AnyResource`: if `T == U`.

        The owner of the resource may freely restrict the resource.

        - Static: Yes
        - Dynamic: Yes

      - When `T == AnyResource`: if the run-time type is `U`.

        - Static: No
        - Dynamic: Yes

  - `AnyResource`:

    - A restricted resource type `T{Us}`
      is a subtype of a restricted resource type `AnyResource{Vs}`:

      - When `T != AnyResource`: if `T` conforms to `Vs`.

        `Us` and `Vs` do *not* have to be subsets.

        - Static: Yes
        - Dynamic: Yes

      - When `T == AnyResource`:

        - Static: if `Vs` is a subset of `Us`
        - Dynamic: if the run-time type conforms to `Vs`

    - An unrestricted resource type `T`
      is a subtype of a restricted resource type `AnyResource{Us}`:

      - When `T != AnyResource`: if `T` conforms to `Us`.

        - Static: Yes
        - Dynamic: Yes

      - When `T == AnyResource`: if the run-time type conforms to `Us`.

        - Static: No
        - Dynamic: Yes

- Supertype: **Unrestricted Resource**:

  - **Not** `AnyResource`:

    - A restricted resource type `T{Us}`
      is a subtype of an unrestricted resource type `V`:

      - When `T != AnyResource`: if `T == V`.

        The owner of the resource may freely unrestrict the resource.

        - Static: Yes
        - Dynamic: Yes

      - When `T == AnyResource`: if the run-time type is `V`.

        - Static: No
        - Dynamic: Yes

    - An unrestricted resource type `T`
      is a subtype of an unrestricted resource type `V`: if `T == V`.

      - Static: Yes
      - Dynamic: Yes

  - `AnyResource`

    - A restricted resource type `T{Us}` or unrestricted resource type `T`
      is a subtype of the type `AnyResource`: always.

      - Static: Yes
      - Dynamic: Yes

## References

- **Authorized**

  An authorized reference type `auth &T` is a subtype of an unauthorized reference type `&U`
  or an authorized reference type `auth &U` if `T` is a subtype of `U`.

  - Static: Yes
  - Dynamic: Yes

- **Unauthorized**

  - An unauthorized reference type `&T` is a subtype of an authorized reference type `auth &T`: never.

    The holder of the reference may not gain more permissions.

    - Static: No
    - Dynamic: No

  - Supertype: **Reference to Restricted Resource**

    - **Not** `AnyResource`:

      - An unauthorized reference to a restricted resource type `&T{Us}`
        is a subtype of a reference to a restricted resource type `&V{Ws}`:

        - When `T != AnyResource`: if `T == V` and `Ws` is a subset of `Us`.

          The holder of the reference may not gain more permissions or knowledge
          and may only further restrict the reference to the resource.

          - Static: Yes
          - Dynamic: Yes

        - When `T == AnyResource`: never.

          The holder of the reference may not gain more permissions or knowledge.

          - Static: No
          - Dynamic: No

      - An unauthorized reference to an unrestricted resource type `&T`
        is a subtype of a reference to a restricted resource type `&U{Vs}`:

        - When `T != AnyResource`: if `T == U`.

          The holder of the reference may only further restrict the reference.

          - Static: Yes
          - Dynamic: Yes

        - When `T == AnyResource`: never.

          The holder of the reference may not gain more permissions or knowledge.

          - Static: No
          - Dynamic: No

    - `AnyResource`:

      - An unauthorized reference to a restricted resource type `&T{Us}`
        is a subtype of a reference to a restricted resource type `&AnyResource{Vs}`: if `Vs` is a subset of `Us`.

        The holder of the reference may only further restrict the reference.

        The requirement for `T` to conform to `Vs` is implied by the subset requirement.

        - Static: Yes
        - Dynamic: Yes

      - An unauthorized reference to an unrestricted resource type `&T`
        is a subtype of a reference to a restricted resource type `&AnyResource{Us}`:

        - When `T != AnyResource`: if `T` conforms to `Us`.

          The holder of the reference may only restrict the reference.

          - Static: Yes
          - Dynamic: Yes

        - When `T == AnyResource`: never.

          The holder of the reference may not gain more permissions or knowledge.

          - Static: No
          - Dynamic: No

  - Supertype: **Unrestricted Resource**:

    - **Not** `AnyResource`:

      - An unauthorized reference to a restricted resource type `&T{Us}`
        is a subtype of a reference to an unrestricted resource type `&V`: never.

        The holder of the reference may not gain more permissions or knowledge.

        - Static: No
        - Dynamic: No

      - An unauthorized reference to an unrestricted resource type `&T`
        is a subtype of a reference to an unrestricted resource type `&V`: if `T == V`.

        - Static: Yes
        - Dynamic: Yes

    - `AnyResource`

      - An unauthorized reference to a restricted resource type `&T{Us}` or
        to a unrestricted resource type `&T`
        is a subtype of the type `&AnyResource`: always.

        - Static: Yes
        - Dynamic: Yes

