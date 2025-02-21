package bridge

import (
    "strings"
)

// TopicMatcher handles MQTT topic pattern matching and substitution
type TopicMatcher struct {
    SourcePattern      string
    DestinationPattern string
    sourceSegments     []string
    destSegments       []string
}

// NewTopicMatcher creates a new topic matcher
func NewTopicMatcher(source, destination string) *TopicMatcher {
    return &TopicMatcher{
        SourcePattern:      source,
        DestinationPattern: destination,
        sourceSegments:     strings.Split(source, "/"),
        destSegments:       strings.Split(destination, "/"),
    }
}

// Match checks if a topic matches the source pattern
func (tm *TopicMatcher) Match(topic string) bool {
    if !strings.Contains(tm.SourcePattern, "+") && !strings.Contains(tm.SourcePattern, "#") {
        return tm.SourcePattern == topic
    }

    topicSegments := strings.Split(topic, "/")

    // Handle # wildcard - matches any number of levels
    if strings.Contains(tm.SourcePattern, "#") {
        return tm.matchHash(topicSegments)
    }

    // Handle + wildcard - matches exactly one level
    return tm.matchPlus(topicSegments)
}

// matchHash handles # wildcard matching
func (tm *TopicMatcher) matchHash(topicSegments []string) bool {
    for i, seg := range tm.sourceSegments {
        if seg == "#" {
            return true // # matches all remaining segments
        }
        if i >= len(topicSegments) || (seg != "+" && seg != topicSegments[i]) {
            return false
        }
    }
    return len(tm.sourceSegments) == len(topicSegments)
}

// matchPlus handles + wildcard matching
func (tm *TopicMatcher) matchPlus(topicSegments []string) bool {
    if len(tm.sourceSegments) != len(topicSegments) {
        return false
    }

    for i, seg := range tm.sourceSegments {
        if seg != "+" && seg != topicSegments[i] {
            return false
        }
    }
    return true
}

// Transform generates a destination topic by substituting wildcards
func (tm *TopicMatcher) Transform(sourceTopic string) string {
    // If no wildcards, return destination as is
    if !strings.Contains(tm.SourcePattern, "+") && !strings.Contains(tm.SourcePattern, "#") {
        return tm.DestinationPattern
    }

    sourceSegments := strings.Split(sourceTopic, "/")
    result := make([]string, len(tm.destSegments))
    copy(result, tm.destSegments)

    // Handle # wildcard
    if strings.Contains(tm.SourcePattern, "#") {
        hashIndex := -1
        for i, seg := range tm.sourceSegments {
            if seg == "#" {
                hashIndex = i
                break
            }
        }
        
        if hashIndex != -1 {
            remaining := strings.Join(sourceSegments[hashIndex:], "/")
            for i, seg := range tm.destSegments {
                if seg == "#" {
                    result[i] = remaining
                }
            }
        }
    }

    // Handle + wildcards
    plusCount := 0
    for i, seg := range tm.sourceSegments {
        if seg == "+" && i < len(sourceSegments) {
            for j, destSeg := range tm.destSegments {
                if destSeg == "+" {
                    if plusCount == countPlusBeforeIndex(tm.sourceSegments, i) {
                        result[j] = sourceSegments[i]
                        break
                    }
                    plusCount++
                }
            }
        }
    }

    return strings.Join(result, "/")
}

// countPlusBeforeIndex counts + wildcards before a given index
func countPlusBeforeIndex(segments []string, index int) int {
    count := 0
    for i := 0; i < index; i++ {
        if segments[i] == "+" {
            count++
        }
    }
    return count
}
