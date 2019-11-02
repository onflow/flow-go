
contract interface PollingPlace {
    resource Ballot {}

    pub fun cast(ballot: <-Ballot)
}

pub contract Voting: PollingPlace {

    pub resource Ballot {
        pub let proposals: [String]
        pub var choice: Int?

        init(proposals: [String]) {
            self.proposals = proposals
            self.choice = nil
        }

        pub fun vote(proposal: Int) {
            pre {
                proposal <= self.proposals.length
            }
            self.choice = proposal
        }
    }

    pub let proposals: [String]
    pub let votes: {String: Int}

    init(proposals: [String]) {
        self.proposals = proposals
        self.votes = {}
    }

    pub fun cast(ballot: <-Ballot) {
        pre {
            ballot.choice != nil:
                "Ballot must have a choice"
        }
        if let choice = ballot.choice {
            let proposal = self.proposals[choice]
            self.votes[proposal] = (self.votes[proposal] ?? 0) + 1
            destroy ballot
        }
    }

    pub fun issueBallot(): <-Ballot {
        return <-create Ballot(proposals: self.proposals)
    }
}
